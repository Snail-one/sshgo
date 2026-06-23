package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func main() {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		fmt.Fprintln(os.Stderr, "lssh only supports Linux and Windows")
		os.Exit(1)
	}

	cfgPath, err := userSSHConfigPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get user home: %v\n", err)
		os.Exit(1)
	}
	hosts, err := parseSSHConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse %s: %v\n", cfgPath, err)
		os.Exit(1)
	}

	for _, host := range hosts {
		fmt.Println(host)
	}
}

func userSSHConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ssh", "config"), nil
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

		fields := splitConfigFields(line)
		if len(fields) == 0 {
			continue
		}
		kw := strings.ToLower(fields[0])

		if kw == "host" && len(fields) > 1 {
			for _, name := range fields[1:] {
				if shouldSkipHostPattern(name) {
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
				if globErr != nil {
					return fmt.Errorf("invalid include pattern %s: %w", inc, globErr)
				}
				if len(matches) == 0 {
					// treat as literal path if no glob match
					matches = []string{incPath}
				}
				for _, m := range matches {
					abs, _ := filepath.Abs(m)
					if visited[abs] {
						continue // cycle prevention
					}
					fi, statErr := os.Stat(m)
					if statErr != nil {
						if os.IsNotExist(statErr) {
							continue
						}
						return fmt.Errorf("stat include %s: %w", m, statErr)
					}
					if fi.IsDir() {
						continue
					}
					visited[abs] = true
					if err := parseConfigFile(m, seen, hosts, visited); err != nil {
						return fmt.Errorf("parse include %s: %w", m, err)
					}
				}
			}
		}
	}
	return scanner.Err()
}

func splitConfigFields(line string) []string {
	fields := strings.Fields(line)
	for i, field := range fields {
		if strings.HasPrefix(field, "#") {
			return fields[:i]
		}
	}
	return fields
}

func shouldSkipHostPattern(name string) bool {
	name = strings.TrimSpace(name)
	return name == "" ||
		name == "*" ||
		strings.HasPrefix(name, "!") ||
		strings.ContainsAny(name, "*?[")
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
