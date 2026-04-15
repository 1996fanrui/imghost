// Package cmd implements the imghost CLI commands.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/1996fanrui/imghost/internal/config"
)

// UnitName is the systemd user unit / launchd label base shared across
// platform adapters. Kept as a single constant so the CLI never hard-codes
// the daemon name in multiple places.
const UnitName = "imghostd"

// NotInstalledMessage is the platform-specific guidance printed when the
// service unit / agent is not installed. REQ-46FE fixes the exact wording
// per platform; it is set in the platform build-tagged service_*.go files.
var NotInstalledMessage string

// configLoader is the hook through which non-version subcommands verify
// that the shared XDG config is readable and retrieve settings (listen_addr,
// api_key) needed to talk to the daemon. Tests swap this out so the CLI
// never touches the real filesystem.
var configLoader = func() (*config.Config, error) {
	return config.Load()
}

var rootCmd = &cobra.Command{
	Use:           "imghost",
	Short:         "imghost CLI",
	Long:          "imghost is the user-facing CLI for the imghostd daemon.",
	SilenceUsage:  true,
	SilenceErrors: true,
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(putCmd)
	rootCmd.AddCommand(getCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(aclCmd)
}

// Execute runs the cobra root command.
func Execute() error {
	return rootCmd.Execute()
}

// requireConfig loads the shared XDG config, surfacing any error so the
// caller can exit non-zero. It is used as a PersistentPreRunE on every
// non-version subcommand to guarantee CLI and daemon see the same config.
func requireConfig(_ *cobra.Command, _ []string) error {
	if _, err := configLoader(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return nil
}
