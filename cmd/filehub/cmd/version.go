package cmd

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is overridden at build time via `-ldflags "-X .../cmd.version=..."`.
var version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the filehub CLI version and build metadata",
	RunE: func(cmd *cobra.Command, _ []string) error {
		commit, date := readBuildInfo()
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "filehub version %s\n", version)
		fmt.Fprintf(out, "commit: %s\n", commit)
		fmt.Fprintf(out, "date: %s\n", date)
		fmt.Fprintf(out, "go: %s\n", runtime.Version())
		return nil
	},
}

// readBuildInfo extracts VCS commit and build date from the Go build info
// embedded in the binary. Returns "unknown" when unavailable (e.g. tests).
func readBuildInfo() (commit, date string) {
	commit, date = "unknown", "unknown"
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if s.Value != "" {
				commit = s.Value
			}
		case "vcs.time":
			if s.Value != "" {
				date = s.Value
			}
		}
	}
	return
}
