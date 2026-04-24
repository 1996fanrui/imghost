//go:build windows

package cmd

import (
	"fmt"
	"io"
)

// windowsGuidance is the platform-specific message printed on every service
// subcommand. Windows has no first-class user service integration in this
// MVP (REQ-46FE); we print guidance and exit 0 so scripts that blindly
// invoke `filehub service <op>` on Windows do not fail.
const windowsGuidance = "Windows has no native user service integration. Run filehubd directly or configure a Task Scheduler job."

func init() {
	adapter = windowsAdapter{}
	NotInstalledMessage = windowsGuidance
}

type windowsAdapter struct{}

// run always prints the guidance and returns nil so every Windows service
// subcommand exits 0 (REQ-46FE).
func (windowsAdapter) run(_ serviceOp, stdout, _ io.Writer) error {
	fmt.Fprintln(stdout, windowsGuidance)
	return nil
}
