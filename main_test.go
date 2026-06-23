package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseSSHConfig_Basic(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	content := `
# comment
Host myserver
    HostName 10.0.0.1
    User root

Host gitlab git
    HostName gitlab.com
    User git

Host *
    # global
`
	if err := os.WriteFile(cfg, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	hosts, err := parseSSHConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"git", "gitlab", "myserver"} // sorted alphabetically
	if !reflect.DeepEqual(hosts, want) {
		t.Errorf("got %v, want %v", hosts, want)
	}
}

func TestParseSSHConfig_WildcardsSkipped(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	content := `
Host *
Host *.internal
Host *.example.com
Host prod-db
`
	mustWriteFile(t, cfg, content)

	hosts, _ := parseSSHConfig(cfg)
	want := []string{"prod-db"}
	if !reflect.DeepEqual(hosts, want) {
		t.Errorf("got %v want %v", hosts, want)
	}
}

func TestParseSSHConfig_CommentsAndNegatedPatterns(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	content := `
Host prod-db # production database
Host prod-web !prod-web-old *.internal
Host !bastion
`
	mustWriteFile(t, cfg, content)

	hosts, err := parseSSHConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"prod-db", "prod-web"}
	if !reflect.DeepEqual(hosts, want) {
		t.Errorf("got %v, want %v", hosts, want)
	}
}

func TestParseSSHConfig_MissingFile(t *testing.T) {
	hosts, err := parseSSHConfig("/non/existent/path/config-lssh-test")
	if err != nil {
		t.Errorf("expected no error for missing file, got %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected empty hosts for missing config, got %v", hosts)
	}
}

func TestParseSSHConfig_Include(t *testing.T) {
	dir := t.TempDir()

	// main config
	mainCfg := filepath.Join(dir, "config")
	mainContent := `
Host main1
    HostName 1.1.1.1

Include includes/*.conf
Include extra.conf
`
	mustWriteFile(t, mainCfg, mainContent)

	// include dir + glob
	incDir := filepath.Join(dir, "includes")
	mustMkdir(t, incDir)
	mustWriteFile(t, filepath.Join(incDir, "a.conf"), "Host inc-a\n    HostName a.local\n")
	mustWriteFile(t, filepath.Join(incDir, "b.conf"), "Host inc-b\n")

	// literal include
	mustWriteFile(t, filepath.Join(dir, "extra.conf"), "Host extra-one\nHost extra-two\n")

	hosts, err := parseSSHConfig(mainCfg)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	want := []string{"extra-one", "extra-two", "inc-a", "inc-b", "main1"}
	if !reflect.DeepEqual(hosts, want) {
		t.Errorf("got %v\nwant %v", hosts, want)
	}
}

func TestParseSSHConfig_IncludeCycleSafety(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "config")
	inc := filepath.Join(dir, "loop.conf")

	mustWriteFile(t, main, "Host root\nInclude loop.conf\n")
	mustWriteFile(t, inc, "Host looped\nInclude config\n")

	hosts, err := parseSSHConfig(main)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// should still collect without infinite loop
	wantContains := map[string]bool{"root": true, "looped": true}
	for _, h := range hosts {
		delete(wantContains, h)
	}
	if len(wantContains) != 0 {
		t.Errorf("missing expected hosts, remaining: %v, got: %v", wantContains, hosts)
	}
}

func TestParseSSHConfig_IncludeParseError(t *testing.T) {
	dir := t.TempDir()
	main := filepath.Join(dir, "config")
	inc := filepath.Join(dir, "bad.conf")

	mustWriteFile(t, main, "Host root\nInclude bad.conf\n")
	mustWriteFile(t, inc, strings.Repeat("a", 70*1024))

	_, err := parseSSHConfig(main)
	if err == nil {
		t.Fatal("expected include parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse include") {
		t.Fatalf("expected include context in error, got %v", err)
	}
}

func TestExpandIncludePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	cases := []struct {
		in       string
		base     string
		contains string // substring check
	}{
		{"~/foo/bar", "/tmp", filepath.Join(home, "foo/bar")},
		{"relative/sub", "/home/user/.ssh", filepath.Join("/home/user/.ssh", "relative/sub")},
		{"/abs/path", "/whatever", "/abs/path"},
	}
	for _, c := range cases {
		got := expandIncludePath(c.in, c.base)
		if got != c.contains {
			t.Errorf("expandIncludePath(%q, %q) = %q, want %q", c.in, c.base, got, c.contains)
		}
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
