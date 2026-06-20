package main

import (
	"os"
	"path/filepath"
	"reflect"
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
	os.WriteFile(cfg, []byte(content), 0644)

	hosts, _ := parseSSHConfig(cfg)
	want := []string{"prod-db"}
	if !reflect.DeepEqual(hosts, want) {
		t.Errorf("got %v want %v", hosts, want)
	}
}

func TestParseSSHConfig_MissingFile(t *testing.T) {
	hosts, err := parseSSHConfig("/non/existent/path/config-sshls-test")
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
	os.WriteFile(mainCfg, []byte(mainContent), 0644)

	// include dir + glob
	incDir := filepath.Join(dir, "includes")
	os.Mkdir(incDir, 0755)
	os.WriteFile(filepath.Join(incDir, "a.conf"), []byte("Host inc-a\n    HostName a.local\n"), 0644)
	os.WriteFile(filepath.Join(incDir, "b.conf"), []byte("Host inc-b\n"), 0644)

	// literal include
	os.WriteFile(filepath.Join(dir, "extra.conf"), []byte("Host extra-one\nHost extra-two\n"), 0644)

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

	os.WriteFile(main, []byte("Host root\nInclude loop.conf\n"), 0644)
	os.WriteFile(inc, []byte("Host looped\nInclude ../config\n"), 0644) // would cycle

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
