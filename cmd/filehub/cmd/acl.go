package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/1996fanrui/filehub/internal/config"
	"github.com/1996fanrui/filehub/internal/permission"
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
		remote := normalizeRemote(args[0])
		u := aclURL(mustConfig(), remote)
		resp, err := httpDo("GET", u, mustConfig().APIKey, nil, "")
		if err != nil {
			return formatCLIError("acl get", remote, err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return formatCLIError("acl get", remote, err)
		}
		var body struct {
			Path   string `json:"path"`
			Access string `json:"access"`
		}
		if err := json.Unmarshal(raw, &body); err != nil || body.Access == "" {
			return formatCLIError("acl get", remote, fmt.Errorf("unexpected response: %s", string(raw)))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", remote, body.Access)
		return nil
	},
}

var aclSetCmd = &cobra.Command{
	Use:               "set <remote-path> <public|private>",
	Short:             "Set the explicit ACL for <remote-path>",
	Args:              cobra.ExactArgs(2),
	PersistentPreRunE: requireConfig,
	RunE: func(cmd *cobra.Command, args []string) error {
		remote := normalizeRemote(args[0])
		access, err := permission.Parse(args[1])
		if err != nil {
			return formatCLIError("acl set", remote, fmt.Errorf("invalid access %q: must be public or private", args[1]))
		}
		body, _ := json.Marshal(map[string]string{"access": string(access)})
		u := aclURL(mustConfig(), remote)
		resp, err := httpDo("PUT", u, mustConfig().APIKey, bytes.NewReader(body), "application/json")
		if err != nil {
			return formatCLIError("acl set", remote, err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		fmt.Fprintf(cmd.OutOrStdout(), "acl set %s = %s\n", remote, access)
		return nil
	},
}

var aclRmCmd = &cobra.Command{
	Use:               "rm <remote-path>",
	Short:             "Remove the explicit ACL for <remote-path>",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: requireConfig,
	RunE: func(cmd *cobra.Command, args []string) error {
		remote := normalizeRemote(args[0])
		u := aclURL(mustConfig(), remote)
		resp, err := httpDo("DELETE", u, mustConfig().APIKey, nil, "")
		if err != nil {
			return formatCLIError("acl rm", remote, err)
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		fmt.Fprintf(cmd.OutOrStdout(), "acl cleared %s\n", remote)
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
