//go:build linux

package cmd

import (
	"errors"
	"io"
	"os/exec"
)

// systemctlUnitMissingExits contains the exit codes that systemctl emits when
// the requested unit is not installed:
//   - 4: unit unknown / not loaded (returned by "status")
//   - 5: requested operation not supported / unit not found (returned by
//     "start", "stop", and other mutating ops against a missing unit)
var systemctlUnitMissingExits = map[int]bool{4: true, 5: true}

func init() {
	adapter = linuxAdapter{}
	NotInstalledMessage = "filehub systemd --user unit is not installed. See the CLI docs for how to install it."
}

type linuxAdapter struct{}

func (linuxAdapter) run(op serviceOp, stdout, stderr io.Writer) error {
	// For "logs" (journalctl), detect a missing unit before streaming,
	// because journalctl exits 0 even when the unit has never been registered.
	if op == opLogs {
		if !isLinuxUnitInstalled() {
			return errServiceNotInstalled
		}
	}
	cmd := buildLinuxCommand(op)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if isSystemctlUnitMissing(op, err) {
			return errServiceNotInstalled
		}
		return err
	}
	return nil
}

// isLinuxUnitInstalled reports whether the filehubd systemd --user unit file
// exists on this host. It uses "systemctl --user cat" which exits non-zero
// when the unit is not found.
func isLinuxUnitInstalled() bool {
	err := exec.Command("systemctl", "--user", "cat", UnitName).Run()
	return err == nil
}

// buildLinuxCommand maps a service op to the concrete systemctl / journalctl
// invocation. Extracted so unit tests can assert the argv shape without
// actually forking a child process.
func buildLinuxCommand(op serviceOp) *exec.Cmd {
	if op == opLogs {
		return exec.Command("journalctl", "--user-unit", UnitName, "-f", "--no-pager")
	}
	return exec.Command("systemctl", "--user", string(op), UnitName)
}

func isSystemctlUnitMissing(op serviceOp, err error) bool {
	var ee *exec.ExitError
	if errors.As(err, &ee) && systemctlUnitMissingExits[ee.ExitCode()] {
		return true
	}
	return false
}
