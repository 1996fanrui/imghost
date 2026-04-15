// Command imghost runs the file hosting HTTP server.
//
// @title                       imghost API
// @version                     1.0
// @description                 Self-hosted file server with per-path ACL.
// @BasePath                    /
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/1996fanrui/imghost/internal/config"
	"github.com/1996fanrui/imghost/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	if err := server.Start(context.Background(), cfg); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}
