package cmd

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/1996fanrui/imghost/internal/config"
)

type fakeAdapter struct {
	calls     []serviceOp
	returnErr error
}

func (f *fakeAdapter) run(op serviceOp, _, _ io.Writer) error {
	f.calls = append(f.calls, op)
	return f.returnErr
}

// withTestEnv swaps the package-level adapter and configLoader with test
// doubles so the CLI never touches the real filesystem or exec machinery.
// The returned cleanup restores the originals.
func withTestEnv(t *testing.T, a serviceAdapter) func() {
	t.Helper()
	origAdapter := adapter
	origLoader := configLoader
	adapter = a
	configLoader = func() (*config.Config, error) { return &config.Config{}, nil }
	return func() {
		adapter = origAdapter
		configLoader = origLoader
	}
}

func runService(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

func TestServiceSubcommandsInvokeAdapter(t *testing.T) {
	for _, op := range []serviceOp{opStart, opStop, opStatus, opLogs} {
		fake := &fakeAdapter{}
		restore := withTestEnv(t, fake)
		if _, err := runService(t, "service", string(op)); err != nil {
			restore()
			t.Fatalf("service %s: %v", op, err)
		}
		restore()
		if len(fake.calls) != 1 || fake.calls[0] != op {
			t.Errorf("expected adapter call %q, got %v", op, fake.calls)
		}
	}
}

func TestServiceNotInstalledPrintsGuidance(t *testing.T) {
	fake := &fakeAdapter{returnErr: errServiceNotInstalled}
	restore := withTestEnv(t, fake)
	defer restore()
	out, err := runService(t, "service", "start")
	if err == nil {
		t.Fatal("expected non-nil error to trigger non-zero exit")
	}
	if !strings.Contains(out, NotInstalledMessage) {
		t.Errorf("expected guidance %q in output: %q", NotInstalledMessage, out)
	}
}

func TestServiceRequireConfigFailure(t *testing.T) {
	restore := withTestEnv(t, &fakeAdapter{})
	defer restore()
	configLoader = func() (*config.Config, error) { return nil, io.ErrUnexpectedEOF }
	if _, err := runService(t, "service", "status"); err == nil {
		t.Fatal("expected config load error to propagate")
	}
}

// TestServiceWindowsExitsZeroForAllSubcommands pins REQ-46FE: on Windows the
// CLI has no native service integration, and every `imghost service <op>`
// must exit 0 — even when the shared config.toml is unreadable. We flip the
// package-level serviceGOOS to simulate Windows and use an adapter that
// returns nil (matching the real windowsAdapter contract).
func TestServiceWindowsExitsZeroForAllSubcommands(t *testing.T) {
	fake := &fakeAdapter{}
	restore := withTestEnv(t, fake)
	defer restore()
	// Simulate a missing / broken config on the host.
	configLoader = func() (*config.Config, error) { return nil, io.ErrUnexpectedEOF }
	origGOOS := serviceGOOS
	serviceGOOS = "windows"
	defer func() { serviceGOOS = origGOOS }()

	for _, op := range []serviceOp{opStart, opStop, opStatus, opLogs} {
		if _, err := runService(t, "service", string(op)); err != nil {
			t.Errorf("windows: service %s returned err %v (expected nil / exit 0)", op, err)
		}
	}
}
