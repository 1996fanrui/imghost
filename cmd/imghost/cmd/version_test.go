package cmd

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	var buf bytes.Buffer
	cmd := versionCmd
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("version run: %v", err)
	}
	out := buf.String()
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, version) {
		t.Errorf("expected version string %q in output: %s", version, out)
	}
	if !strings.Contains(out, runtime.Version()) {
		t.Errorf("expected Go version %q in output: %s", runtime.Version(), out)
	}
	for _, key := range []string{"commit:", "date:", "go:"} {
		if !strings.Contains(out, key) {
			t.Errorf("expected %q in output: %s", key, out)
		}
	}
}
