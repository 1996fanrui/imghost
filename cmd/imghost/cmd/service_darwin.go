//go:build darwin

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// launchdLabel is the launchd service label used by the plist. The plist
// itself is delivered by a separate install workflow (see CLI docs).
const launchdLabel = "com.imghost.imghostd"

func init() {
	adapter = darwinAdapter{}
	NotInstalledMessage = "imghost launchd agent is not installed. See the CLI docs for how to install it."
}

type darwinAdapter struct{}

func (d darwinAdapter) run(op serviceOp, stdout, stderr io.Writer) error {
	cmd, err := buildDarwinCommand(op)
	if err != nil {
		return err
	}
	// Tee stderr into a buffer so we can inspect launchctl's diagnostic
	// message to distinguish "agent not installed" from real failures
	// (permission denied, plist syntax error, etc.). Without this, every
	// non-zero exit would be mis-translated to errServiceNotInstalled and
	// hide the actual cause from the user.
	var errBuf bytes.Buffer
	cmd.Stdout = stdout
	cmd.Stderr = io.MultiWriter(stderr, &errBuf)
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			if isLaunchctlUnitMissing(errBuf.String()) {
				return errServiceNotInstalled
			}
		}
		return err
	}
	return nil
}

// launchctlUnitMissingMarkers are substrings launchctl prints on stderr when
// the requested service is not registered on the host. Sourced from the
// launchctl(1) man page and observed macOS 12–14 output for bootstrap /
// bootout / print against an unknown target.
var launchctlUnitMissingMarkers = []string{
	"Could not find",
	"No such process",
	"could not find service",
	"not loaded",
	"Service is disabled",
}

// isLaunchctlUnitMissing reports whether launchctl's stderr indicates the
// target service is simply not installed (as opposed to a real failure such
// as a plist syntax error or a permissions problem).
func isLaunchctlUnitMissing(stderr string) bool {
	for _, m := range launchctlUnitMissingMarkers {
		if strings.Contains(stderr, m) {
			return true
		}
	}
	return false
}

// buildDarwinCommand maps a service op to launchctl bootstrap/bootout/print
// (macOS 10.11+) or `log show` for logs. Uses gui/<uid> domain for user
// services, matching systemd --user semantics on Linux.
func buildDarwinCommand(op serviceOp) (*exec.Cmd, error) {
	uid := os.Getuid()
	domain := fmt.Sprintf("gui/%d", uid)
	target := fmt.Sprintf("%s/%s", domain, launchdLabel)
	switch op {
	case opStart:
		plist, err := plistPath()
		if err != nil {
			return nil, err
		}
		if _, statErr := os.Stat(plist); errors.Is(statErr, os.ErrNotExist) {
			return nil, errServiceNotInstalled
		}
		return exec.Command("launchctl", "bootstrap", domain, plist), nil
	case opStop:
		return exec.Command("launchctl", "bootout", target), nil
	case opStatus:
		return exec.Command("launchctl", "print", target), nil
	case opLogs:
		return exec.Command("log", "show",
			"--predicate", fmt.Sprintf("subsystem == %q", launchdLabel),
			"--last", "1h"), nil
	default:
		return nil, fmt.Errorf("unknown service op: %s", op)
	}
}

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}
