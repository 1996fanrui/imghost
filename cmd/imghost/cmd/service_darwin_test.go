//go:build darwin

package cmd

import "testing"

// TestIsLaunchctlUnitMissing pins the stderr classification that the darwin
// adapter relies on to distinguish "agent not installed" from real failures
// (permission denied, plist syntax errors, etc.). Without this
// differentiation, every non-zero launchctl exit would be mis-translated to
// errServiceNotInstalled and hide the real cause from the user.
func TestIsLaunchctlUnitMissing(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   bool
	}{
		{"bootstrap not found", "Could not find service\n", true},
		{"print no such process", "launchctl: No such process\n", true},
		{"bootout not loaded", "Boot-out failed: not loaded\n", true},
		{"disabled", "Service is disabled\n", true},
		{"permission denied is real error", "launchctl: Permission denied\n", false},
		{"plist syntax is real error", "Load failed: 5: Input/output error\n", false},
		{"empty stderr is real error", "", false},
	}
	for _, c := range cases {
		if got := isLaunchctlUnitMissing(c.stderr); got != c.want {
			t.Errorf("%s: isLaunchctlUnitMissing(%q) = %v, want %v", c.name, c.stderr, got, c.want)
		}
	}
}
