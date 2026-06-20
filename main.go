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

	// We only require Windows when we are about to actually spawn a terminal
	willLaunch := len(args) > 0 || !list

	if willLaunch && runtime.GOOS != "windows" {
		// Allow menu and --list on any platform (great for development/testing).
		// Actual ssh launch will be simulated.
		fmt.Println("[dev] Running on non-Windows — terminal launches will be simulated (no new window).")
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

	// interactive main menu loop
	fmt.Println("sshls - SSH Host Picker")
	fmt.Printf("Found %d hosts in %s\n", len(hosts), cfgPath)
	fmt.Println("Choose a host to launch in a new terminal window. Menu will return after launch.")
	fmt.Println("Type r to refresh list, 0 or q to exit.")

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println()
		for i, h := range hosts {
			fmt.Printf("  %2d) %s\n", i+1, h)
		}
		fmt.Printf("  %2d) %s\n", 0, "退出")
		fmt.Println()

		choice := chooseHostOnce(hosts, reader)
		if choice == "__REFRESH__" {
			continue
		}
		if choice == "" {
			fmt.Println("Exiting sshls. Goodbye!")
			break
		}

		launchInTerminal(choice, cfgPath)
		fmt.Printf("\n[✓] Launched new terminal: ssh %s\n", choice)
		fmt.Println("    Returned to main menu. Select another host or press q to quit.")
	}
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

// chooseHostOnce prompts the user once (with re-prompt on invalid input).
// Returns:
//   - host alias string when a host is selected
//   - "__REFRESH__" when user wants to refresh the list (type r)
//   - "" to quit (q, 0, empty, etc.)
// It does NOT print the menu list itself.
func chooseHostOnce(hosts []string, reader *bufio.Reader) string {
	for {
		fmt.Printf("Enter number (1-%d), partial name, r (refresh), 0 or q (quit): ", len(hosts))
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("\nInput error, cancelling.")
			return ""
		}
		input = strings.TrimSpace(input)
		lower := strings.ToLower(input)

		// Special commands first
		if lower == "r" || lower == "refresh" {
			return "__REFRESH__"
		}
		if input == "" || lower == "q" || lower == "quit" || lower == "0" || lower == "exit" {
			return "" // quit
		}

		// Try numeric selection
		if num, err := strconv.Atoi(input); err == nil {
			if num == 0 {
				return ""
			}
			if num >= 1 && num <= len(hosts) {
				return hosts[num-1]
			}
			fmt.Printf("Number out of range (1-%d). Try again.\n", len(hosts))
			continue
		}

		// Substring / name match (case-insensitive, first match)
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

	if runtime.GOOS != "windows" {
		// Simulation for development / testing on Linux/macOS
		fmt.Printf("  [SIMULATED] Would open new terminal and run: ssh %s\n", alias)
		fmt.Printf("  (On real Windows this opens wt/cmd and executes the ssh command.)\n")
		return
	}

	var cmd *exec.Cmd

	if _, err := exec.LookPath("wt"); err == nil {
		// 直接调用 Windows Terminal（更干净，不经过 cmd.exe）
		// 使用 new-tab 可以设置标题，并且 wt 会自动在新窗口/标签中运行
		title := "SSH: " + alias
		cmd = exec.Command("wt", "new-tab", "--title", title, "ssh", alias)
	} else {
		// 传统 cmd 窗口必须用 start 来打开新窗口
		title := "SSH: " + alias
		cmd = exec.Command("cmd.exe", "/c", "start", title, "cmd.exe", "/k", "ssh", alias)
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch terminal: %v\n", err)
		fmt.Fprintf(os.Stderr, "You can still run manually: ssh %s\n", alias)
	}
	// Fire-and-forget: do not wait
}
