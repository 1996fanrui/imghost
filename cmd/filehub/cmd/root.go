// Package cmd implements the filehub CLI commands.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/1996fanrui/filehub/internal/config"
)

// UnitName is the systemd user unit / launchd label base shared across
// platform adapters. Kept as a single constant so the CLI never hard-codes
// the daemon name in multiple places.
const UnitName = "filehubd"

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

// cachedConfig holds the config loaded once per CLI invocation via
// requireConfig, so HTTP subcommands do not re-read (and re-validate) the
// TOML file on every RunE call.
var cachedConfig *config.Config

var rootCmd = &cobra.Command{
	Use:           "filehub",
	Short:         "filehub CLI",
	Long:          "filehub is the user-facing CLI for the filehubd daemon.",
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
	cfg, err := configLoader()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	cachedConfig = cfg
	return nil
}

// mustConfig returns the config loaded by requireConfig. HTTP subcommands
// should call this from their RunE to avoid re-loading the TOML file.
func mustConfig() *config.Config {
	return cachedConfig
}
