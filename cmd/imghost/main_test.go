package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildBinary builds the imghost binary once per test run.
func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "imghost")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v", err)
	}
	return bin
}

type envCase struct {
	name string
	env  map[string]string
	wantStderrSubstr string
}

func TestMainConfigValidationExitsOne(t *testing.T) {
	bin := buildBinary(t)

	// A writable data dir we reuse.
	dataDir := t.TempDir()
	// A read-only data dir.
	readonly := t.TempDir()
	if err := os.Chmod(readonly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readonly, 0o755) })

	cases := []envCase{
		{
			name: "missing API_KEY",
			env:  map[string]string{"DATA_DIR": dataDir},
			wantStderrSubstr: "API_KEY",
		},
		{
			name: "bad DEFAULT_ACCESS",
			env:  map[string]string{"API_KEY": "k", "DEFAULT_ACCESS": "foo", "DATA_DIR": dataDir},
			wantStderrSubstr: "DEFAULT_ACCESS",
		},
		{
			name: "non-numeric PORT",
			env:  map[string]string{"API_KEY": "k", "PORT": "abc", "DATA_DIR": dataDir},
			wantStderrSubstr: "PORT",
		},
		{
			name: "out-of-range PORT",
			env:  map[string]string{"API_KEY": "k", "PORT": "70000", "DATA_DIR": dataDir},
			wantStderrSubstr: "PORT",
		},
		{
			name: "DATA_DIR equals reserved",
			env:  map[string]string{"API_KEY": "k", "DATA_DIR": "/var/lib/imghost"},
			wantStderrSubstr: "DATA_DIR",
		},
		{
			name: "DATA_DIR under reserved",
			env:  map[string]string{"API_KEY": "k", "DATA_DIR": "/var/lib/imghost/sub"},
			wantStderrSubstr: "DATA_DIR",
		},
		{
			name: "DATA_DIR missing",
			env:  map[string]string{"API_KEY": "k", "DATA_DIR": filepath.Join(t.TempDir(), "nonexistent")},
			wantStderrSubstr: "DATA_DIR",
		},
		{
			name: "DATA_DIR readonly",
			env:  map[string]string{"API_KEY": "k", "DATA_DIR": readonly},
			wantStderrSubstr: "DATA_DIR",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(bin)
			cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
			for k, v := range c.env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
			stderr, _ := cmd.CombinedOutput()
			if cmd.ProcessState == nil {
				t.Fatal("no process state")
			}
			code := cmd.ProcessState.ExitCode()
			if code != 1 {
				t.Fatalf("exit code %d want 1; stderr: %s", code, stderr)
			}
			if c.wantStderrSubstr != "" && !strings.Contains(string(stderr), c.wantStderrSubstr) {
				t.Fatalf("stderr %q missing %q", stderr, c.wantStderrSubstr)
			}
		})
	}
}
