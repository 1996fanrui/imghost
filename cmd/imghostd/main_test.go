package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "imghostd")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build: %v", err)
	}
	return bin
}

// runWith invokes the built daemon with XDG_CONFIG_HOME pointing at cfgHome
// and returns exit code + combined output.
func runWith(t *testing.T, bin, cfgHome string, extraArgs ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(bin, extraArgs...)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"XDG_CONFIG_HOME=" + cfgHome,
		"XDG_STATE_HOME=" + t.TempDir(),
	}
	out, _ := cmd.CombinedOutput()
	if cmd.ProcessState == nil {
		t.Fatal("no process state")
	}
	return cmd.ProcessState.ExitCode(), string(out)
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	cfgHome := t.TempDir()
	dir := filepath.Join(cfgHome, "imghost")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgHome
}

func TestMainMissingConfigExits1(t *testing.T) {
	bin := buildBinary(t)
	cfgHome := t.TempDir()
	// Ensure no config file exists by pointing at an empty XDG dir without
	// pre-populating imghost/config.toml. Reading must fail because
	// xdg.ConfigFile returns a path inside a freshly-created parent dir, but
	// open will fail since the file itself does not exist.
	code, out := runWith(t, bin, cfgHome)
	if code != 1 {
		t.Fatalf("exit %d out=%s", code, out)
	}
	if !strings.Contains(out, "config") {
		t.Fatalf("stderr missing config: %s", out)
	}
}

func TestMainInvalidConfigExits1(t *testing.T) {
	bin := buildBinary(t)
	cfgHome := writeConfigFile(t, "not = a = valid toml\n")
	code, _ := runWith(t, bin, cfgHome)
	if code != 1 {
		t.Fatalf("exit %d want 1", code)
	}
}

func TestMainNoRootsExits1(t *testing.T) {
	bin := buildBinary(t)
	cfgHome := writeConfigFile(t, `listen_addr = ":0"`+"\n")
	code, out := runWith(t, bin, cfgHome)
	if code != 1 {
		t.Fatalf("exit %d out=%s", code, out)
	}
	if !strings.Contains(out, "root") {
		t.Fatalf("stderr missing root: %s", out)
	}
}

func TestMainUnexpectedArgExits2(t *testing.T) {
	bin := buildBinary(t)
	cfgHome := t.TempDir()
	code, _ := runWith(t, bin, cfgHome, "extra")
	if code != 2 {
		t.Fatalf("exit %d want 2", code)
	}
}

func TestMainUnknownFlagExits2(t *testing.T) {
	bin := buildBinary(t)
	cfgHome := t.TempDir()
	code, _ := runWith(t, bin, cfgHome, "--config", "foo")
	if code != 2 {
		t.Fatalf("exit %d want 2", code)
	}
}
