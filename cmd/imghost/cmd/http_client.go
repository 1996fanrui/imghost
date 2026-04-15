package cmd

import (
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
// non-2xx responses it returns an error carrying the server's response body
// so users see the daemon's structured error (apierror.Response) verbatim.
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
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return resp, nil
}
