package cmd

import (
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

var putCmd = &cobra.Command{
	Use:               "put <remote-path> <local-file>",
	Short:             "Upload a local file to the daemon at <remote-path>",
	Args:              cobra.ExactArgs(2),
	PersistentPreRunE: requireConfig,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := configLoader()
		if err != nil {
			return err
		}
		remote := normalizeRemote(args[0])
		local := args[1]
		f, err := os.Open(local)
		if err != nil {
			return err
		}
		defer f.Close()
		u := baseURL(cfg) + (&url.URL{Path: remote}).EscapedPath()
		resp, err := httpDo("PUT", u, cfg.APIKey, f, "application/octet-stream")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, _ = io.Copy(cmd.OutOrStdout(), resp.Body)
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
			cfg, err := configLoader()
			if err != nil {
				return err
			}
			remote := normalizeRemote(args[0])
			u := baseURL(cfg) + (&url.URL{Path: remote}).EscapedPath()
			resp, err := httpDo("GET", u, cfg.APIKey, nil, "")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			var dst io.Writer = cmd.OutOrStdout()
			if getOutput != "" && getOutput != "-" {
				f, err := os.Create(getOutput)
				if err != nil {
					return err
				}
				defer f.Close()
				dst = f
			}
			_, err = io.Copy(dst, resp.Body)
			return err
		},
	}
)

func init() {
	getCmd.Flags().StringVarP(&getOutput, "output", "o", "", "write response body to file (default: stdout)")
}

var rmCmd = &cobra.Command{
	Use:               "rm <remote-path>",
	Short:             "Delete <remote-path> on the daemon",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: requireConfig,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := configLoader()
		if err != nil {
			return err
		}
		remote := normalizeRemote(args[0])
		u := baseURL(cfg) + (&url.URL{Path: remote}).EscapedPath()
		resp, err := httpDo("DELETE", u, cfg.APIKey, nil, "")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		fmt.Fprintln(cmd.OutOrStdout(), resp.Status)
		return nil
	},
}
