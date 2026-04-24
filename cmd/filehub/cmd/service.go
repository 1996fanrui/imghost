package cmd

import (
	"fmt"
	"io"
	"runtime"

	"github.com/spf13/cobra"
)

// serviceOp enumerates the high-level service operations the CLI exposes.
// Platform adapters translate each op into concrete system calls.
type serviceOp string

const (
	opStart  serviceOp = "start"
	opStop   serviceOp = "stop"
	opStatus serviceOp = "status"
	opLogs   serviceOp = "logs"
)

// serviceAdapter is the platform-specific backend for service management.
// It is swapped out in tests via the package-level `adapter` variable.
type serviceAdapter interface {
	// run executes the given op, streaming output to stdout/stderr.
	// On a missing unit / plist, it must return errServiceNotInstalled
	// so the top-level command can print the canonical guidance.
	run(op serviceOp, stdout, stderr io.Writer) error
}

// errServiceNotInstalled is returned by adapters when the service unit or
// plist is not registered on the host. The service command translates it
// into the canonical NotInstalledMessage + non-zero exit.
var errServiceNotInstalled = fmt.Errorf("service not installed")

// adapter is the live service adapter. Platform-specific build-tagged
// files (service_linux.go, service_darwin.go, service_windows.go) set
// the concrete value for the current OS.
var adapter serviceAdapter

var serviceCmd = &cobra.Command{
	Use:               "service",
	Short:             "Manage the filehubd background service",
	PersistentPreRunE: serviceConfigGate,
}

// serviceConfigGate enforces REQ-46FE: on Windows the CLI has no native
// service integration, so every `filehub service <op>` must exit 0 with a
// guidance line — including when the shared config.toml is missing. Gating
// requireConfig behind the non-Windows check guarantees the Windows adapter
// reaches its guidance path regardless of config state.
func serviceConfigGate(cmd *cobra.Command, args []string) error {
	if serviceGOOS == "windows" {
		return nil
	}
	return requireConfig(cmd, args)
}

// serviceGOOS is the platform identifier consulted by serviceConfigGate.
// Kept as a package-level var (not a direct runtime.GOOS reference) so tests
// can flip it to "windows" without a build-tagged file to verify REQ-46FE's
// "every service subcommand exits 0 on Windows" contract.
var serviceGOOS = runtime.GOOS

func init() {
	for _, op := range []serviceOp{opStart, opStop, opStatus, opLogs} {
		serviceCmd.AddCommand(newServiceSubcommand(op))
	}
}

func newServiceSubcommand(op serviceOp) *cobra.Command {
	return &cobra.Command{
		Use:   string(op),
		Short: fmt.Sprintf("%s the filehubd service", op),
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := adapter.run(op, cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err == errServiceNotInstalled {
				fmt.Fprintln(cmd.OutOrStdout(), NotInstalledMessage)
				return err
			}
			return err
		},
	}
}
