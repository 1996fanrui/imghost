package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"

	"github.com/spf13/cobra"
)

var putCmd = &cobra.Command{
	Use:               "put <remote-path> <local-file>",
	Short:             "Upload a local file to the daemon at <remote-path>",
	Args:              cobra.ExactArgs(2),
	PersistentPreRunE: requireConfig,
	RunE: func(cmd *cobra.Command, args []string) error {
		remote := normalizeRemote(args[0])
		local := args[1]
		info, err := os.Stat(local)
		if err != nil {
			return formatCLIError("put", remote, err)
		}
		if info.IsDir() {
			return formatCLIError("put", remote, fmt.Errorf("%s is a directory", local))
		}
		f, err := os.Open(local)
		if err != nil {
			return formatCLIError("put", remote, err)
		}
		defer f.Close()
		u := baseURL(mustConfig()) + (&url.URL{Path: remote}).EscapedPath()
		resp, err := httpDo("PUT", u, mustConfig().APIKey, f, "application/octet-stream")
		if err != nil {
			return formatCLIError("put", remote, err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		fmt.Fprintf(cmd.OutOrStdout(), "uploaded %s (%s)\n", remote, humanBytes(info.Size()))
		return nil
	},
}

var (
	getOutput string

	getCmd = &cobra.Command{
		Use:               "get <remote-path>",
		Short:             "Download <remote-path> from the daemon",
		Args:              cobra.ExactArgs(1),
		PersistentPreRunE: requireConfig,
		RunE: func(cmd *cobra.Command, args []string) error {
			remote := normalizeRemote(args[0])
			u := baseURL(mustConfig()) + (&url.URL{Path: remote}).EscapedPath()
			resp, err := httpDo("GET", u, mustConfig().APIKey, nil, "")
			if err != nil {
				return formatCLIError("get", remote, err)
			}
			defer resp.Body.Close()

			// Output routing:
			//   -o <path>  → that file
			//   -o -       → stdout
			//   (unset)    → ./<basename of remote>
			target := getOutput
			if target == "" {
				target = "./" + path.Base(remote)
			}

			if target == "-" {
				n, err := io.Copy(cmd.OutOrStdout(), resp.Body)
				if err != nil {
					return formatCLIError("get", remote, err)
				}
				// Stdout mode stays silent on stderr so the user can pipe
				// the bytes. The byte count is useful but not user-facing
				// here; keep stdout pristine.
				_ = n
				return nil
			}

			f, err := os.Create(target)
			if err != nil {
				return formatCLIError("get", remote, err)
			}
			defer f.Close()
			n, err := io.Copy(f, resp.Body)
			if err != nil {
				return formatCLIError("get", remote, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved %s -> %s (%s)\n", remote, target, humanBytes(n))
			return nil
		},
	}
)

func init() {
	getCmd.Flags().StringVarP(&getOutput, "output", "o", "", "write to file (default ./<basename>); use - for stdout")
}

var rmCmd = &cobra.Command{
	Use:               "rm <remote-path>",
	Short:             "Delete <remote-path> on the daemon",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: requireConfig,
	RunE: func(cmd *cobra.Command, args []string) error {
		remote := normalizeRemote(args[0])
		u := baseURL(mustConfig()) + (&url.URL{Path: remote}).EscapedPath()
		resp, err := httpDo("DELETE", u, mustConfig().APIKey, nil, "")
		if err != nil {
			return formatCLIError("rm", remote, err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", remote)
		return nil
	},
}
