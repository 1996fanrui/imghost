package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/1996fanrui/imghost/internal/config"
)

// baseURL derives the daemon URL from cfg.ListenAddr. A leading ":port"
// (the config default) is interpreted as 127.0.0.1:port because the daemon
// binds all interfaces but a CLI on the same host always reaches it via
// loopback.
func baseURL(cfg *config.Config) string {
	addr := cfg.ListenAddr
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	return "http://" + addr
}

// normalizeRemote ensures the remote path starts with exactly one slash.
func normalizeRemote(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

// httpDo executes an authenticated HTTP request against the daemon. On
// non-2xx responses it returns an error whose message is the server's
// apierror.Response.message field (parsed from JSON); callers wrap it with
// operation context via formatCLIError.
func httpDo(method, url, apiKey string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, parseServerError(resp)
	}
	return resp, nil
}

// parseServerError reads the response body and extracts a concise, human
// message from the server's structured error (apierror.Response). If the
// body is not a recognizable JSON error, falls back to the HTTP status.
func parseServerError(resp *http.Response) error {
	raw, _ := io.ReadAll(resp.Body)
	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &body); err == nil && body.Message != "" {
		return fmt.Errorf("%s", body.Message)
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed != "" {
		return fmt.Errorf("%s: %s", resp.Status, trimmed)
	}
	return fmt.Errorf("%s", resp.Status)
}

// formatCLIError produces the unified CLI error format:
//
//	imghost: <op> <target>: <reason>
//
// Cobra prints the returned error to stderr; SilenceErrors on the root
// prevents the default "Error: ..." prefix so this is the only line shown.
func formatCLIError(op, target string, err error) error {
	return fmt.Errorf("imghost: %s %s: %w", op, target, err)
}

// humanBytes renders a byte count as a short, human-readable string.
// Used in success messages so users see "uploaded /a.txt (1.4 KiB)" rather
// than raw byte counts.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
