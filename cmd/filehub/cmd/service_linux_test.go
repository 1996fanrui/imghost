//go:build linux

package cmd

import (
	"os/exec"
	"reflect"
	"testing"
)

func TestBuildLinuxCommandStart(t *testing.T) {
	cmd := buildLinuxCommand(opStart)
	want := []string{"systemctl", "--user", "start", UnitName}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("start args: got %v, want %v", cmd.Args, want)
	}
}

func TestBuildLinuxCommandLogs(t *testing.T) {
	cmd := buildLinuxCommand(opLogs)
	want := []string{"journalctl", "--user-unit", UnitName, "-f", "--no-pager"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("logs args: got %v, want %v", cmd.Args, want)
	}
}

func TestBuildLinuxCommandStopStatus(t *testing.T) {
	for _, op := range []serviceOp{opStop, opStatus} {
		cmd := buildLinuxCommand(op)
		want := []string{"systemctl", "--user", string(op), UnitName}
		if !reflect.DeepEqual(cmd.Args, want) {
			t.Errorf("%s args: got %v, want %v", op, cmd.Args, want)
		}
	}
}

// TestIsSystemctlUnitMissingExitCodes verifies that both exit code 4
// (unit unknown / not loaded, returned by "status") and exit code 5
// (unit not found, returned by "start" and "stop") are recognised as
// "unit not installed" so the adapter surfaces errServiceNotInstalled.
func TestIsSystemctlUnitMissingExitCodes(t *testing.T) {
	// Helper: build a fake *exec.ExitError with the desired exit code by
	// running a shell one-liner that exits with that code.
	fakeExitError := func(code int) error {
		cmd := exec.Command("sh", "-c", "exit "+string(rune('0'+code)))
		err := cmd.Run()
		if err == nil {
			t.Fatalf("expected non-zero exit from sh -c 'exit %d'", code)
		}
		return err
	}

	for _, code := range []int{4, 5} {
		err := fakeExitError(code)
		if !isSystemctlUnitMissing(opStart, err) {
			t.Errorf("exit code %d should be recognised as unit-missing", code)
		}
	}
	// Exit code 1 (generic failure) must NOT be treated as unit-missing.
	err := fakeExitError(1)
	if isSystemctlUnitMissing(opStart, err) {
		t.Error("exit code 1 must not be recognised as unit-missing")
	}
}
