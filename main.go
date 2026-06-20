package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

func main() {
	var list bool
	flag.BoolVar(&list, "list", false, "List all hosts from config and exit")
	flag.Parse()

	// Only guard actual connection launch paths (allows --help / --list / source inspection cross-platform)
	needsWindows := false

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get user home: %v\n", err)
		os.Exit(1)
	}
	cfgPath := filepath.Join(home, ".ssh", "config")

	hosts, parseErr := parseSSHConfig(cfgPath)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: error parsing config: %v\n", parseErr)
	}

	args := flag.Args()

	if list || len(args) > 0 || (len(hosts) > 0) {
		// We will do something interactive or direct that eventually launches (except pure --list)
		if len(args) > 0 || !list {
			needsWindows = true
		}
	}

	if needsWindows && runtime.GOOS != "windows" {
		fmt.Println("This tool is designed for Windows only (terminal launching).")
		fmt.Println("You can still use --list and the parser on other platforms for development.")
		os.Exit(1)
	}

	if len(hosts) == 0 {
		fmt.Printf("No SSH hosts found in %s\n", cfgPath)
		fmt.Println("Create or edit the file with 'Host' entries (not just wildcards) and try again.")
		return
	}

	if list {
		for _, h := range hosts {
			fmt.Println(h)
		}
		return
	}

	if len(args) > 0 {
		alias := args[0]
		launchInTerminal(alias, cfgPath)
		return
	}

	// interactive
	alias := promptSelect(hosts, cfgPath)
	if alias == "" {
		return
	}
	launchInTerminal(alias, cfgPath)
}

func parseSSHConfig(path string) ([]string, error) {
	seen := make(map[string]struct{})
	var hosts []string

	abs, _ := filepath.Abs(path)
	visited := map[string]bool{abs: true}

	// If main config doesn't exist, return empty (not error)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return hosts, nil
	}

	err := parseConfigFile(path, seen, &hosts, visited)
	if err != nil {
		return hosts, err
	}

	sort.Strings(hosts)
	return hosts, nil
}

func parseConfigFile(path string, seen map[string]struct{}, hosts *[]string, visited map[string]bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err // caller decides
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on whitespace. First token is keyword.
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		kw := strings.ToLower(fields[0])

		if kw == "host" && len(fields) > 1 {
			for _, name := range fields[1:] {
				name = strings.TrimSpace(name)
				if name == "" || name == "*" || strings.ContainsAny(name, "*?[") {
					continue
				}
				if _, ok := seen[name]; !ok {
					seen[name] = struct{}{}
					*hosts = append(*hosts, name)
				}
			}
		} else if kw == "include" && len(fields) > 1 {
			baseDir := filepath.Dir(path)
			for _, inc := range fields[1:] {
				incPath := expandIncludePath(inc, baseDir)
				matches, globErr := filepath.Glob(incPath)
				if globErr != nil || len(matches) == 0 {
					// treat as literal path if no glob match
					matches = []string{incPath}
				}
				for _, m := range matches {
					abs, _ := filepath.Abs(m)
					if visited[abs] {
						continue // cycle prevention
					}
					if fi, statErr := os.Stat(m); statErr == nil && !fi.IsDir() {
						visited[abs] = true
						_ = parseConfigFile(m, seen, hosts, visited) // ignore sub errors
					}
				}
			}
		}
	}
	return scanner.Err()
}

func expandIncludePath(p, baseDir string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	// Handle ~/
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	} else if !filepath.IsAbs(p) {
		p = filepath.Join(baseDir, p)
	}
	return p
}

func promptSelect(hosts []string, cfgPath string) string {
	fmt.Println("sshls - SSH Host Picker")
	fmt.Printf("Found %d hosts in %s\n\n", len(hosts), cfgPath)

	for i, h := range hosts {
		fmt.Printf("  %2d) %s\n", i+1, h)
	}
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter number or partial name (q to quit): ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("\nInput error, cancelling.")
			return ""
		}
		input = strings.TrimSpace(input)

		if input == "" || strings.EqualFold(input, "q") {
			fmt.Println("Cancelled.")
			return ""
		}

		// Try numeric selection
		if num, err := strconv.Atoi(input); err == nil {
			if num >= 1 && num <= len(hosts) {
				return hosts[num-1]
			}
			fmt.Printf("Number out of range (1-%d). Try again.\n", len(hosts))
			continue
		}

		// Substring / name match (case-insensitive, first match)
		lower := strings.ToLower(input)
		for _, h := range hosts {
			if strings.Contains(strings.ToLower(h), lower) {
				return h
			}
		}

		fmt.Printf("No match for %q. Try a number or different name.\n", input)
	}
}

func launchInTerminal(alias string, cfgPath string) {
	fmt.Printf("Launching terminal for: ssh %s\n", alias)

	var cmd *exec.Cmd

	if _, err := exec.LookPath("wt"); err == nil {
		// Prefer Windows Terminal
		// start "SSH: alias" wt ssh alias
		title := "SSH: " + alias
		cmd = exec.Command("cmd.exe", "/c", "start", title, "wt", "ssh", alias)
	} else {
		// Classic cmd fallback (keeps window after ssh exits via /k)
		title := "SSH: " + alias
		cmd = exec.Command("cmd.exe", "/c", "start", title, "cmd.exe", "/k", "ssh", alias)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch terminal: %v\n", err)
		fmt.Fprintf(os.Stderr, "You can still run manually: ssh %s\n", alias)
	}
	// Do not wait. Let sshls exit.
}
