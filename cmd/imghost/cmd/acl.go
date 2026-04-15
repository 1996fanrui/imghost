package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/1996fanrui/imghost/internal/config"
	"github.com/1996fanrui/imghost/internal/permission"
)

var aclCmd = &cobra.Command{
	Use:   "acl",
	Short: "Manage per-path ACL overrides",
}

var aclGetCmd = &cobra.Command{
	Use:               "get <remote-path>",
	Short:             "Read the explicit ACL for <remote-path>",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: requireConfig,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := configLoader()
		if err != nil {
			return err
		}
		u := aclURL(cfg, args[0])
		resp, err := httpDo("GET", u, cfg.APIKey, nil, "")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
		if err == nil {
			fmt.Fprintln(cmd.OutOrStdout())
		}
		return err
	},
}

var aclSetCmd = &cobra.Command{
	Use:               "set <remote-path> <public|private>",
	Short:             "Set the explicit ACL for <remote-path>",
	Args:              cobra.ExactArgs(2),
	PersistentPreRunE: requireConfig,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := configLoader()
		if err != nil {
			return err
		}
		access, err := permission.Parse(args[1])
		if err != nil {
			return fmt.Errorf("invalid access %q: must be public or private", args[1])
		}
		body, _ := json.Marshal(map[string]string{"access": string(access)})
		u := aclURL(cfg, args[0])
		resp, err := httpDo("PUT", u, cfg.APIKey, bytes.NewReader(body), "application/json")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		fmt.Fprintln(cmd.OutOrStdout(), resp.Status)
		return nil
	},
}

var aclRmCmd = &cobra.Command{
	Use:               "rm <remote-path>",
	Short:             "Remove the explicit ACL for <remote-path>",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: requireConfig,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := configLoader()
		if err != nil {
			return err
		}
		u := aclURL(cfg, args[0])
		resp, err := httpDo("DELETE", u, cfg.APIKey, nil, "")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		fmt.Fprintln(cmd.OutOrStdout(), resp.Status)
		return nil
	},
}

func init() {
	aclCmd.AddCommand(aclGetCmd, aclSetCmd, aclRmCmd)
}

// aclURL builds "<base>/<remote-path>?acl" with the path properly escaped.
func aclURL(cfg *config.Config, remote string) string {
	u := url.URL{Path: normalizeRemote(remote), RawQuery: "acl"}
	return baseURL(cfg) + u.EscapedPath() + "?" + u.RawQuery
}
