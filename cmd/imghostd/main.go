// Command imghostd runs the file hosting HTTP server.
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
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/1996fanrui/imghost/internal/config"
	"github.com/1996fanrui/imghost/internal/server"
)

func main() {
	flag.Parse()
	if len(flag.Args()) > 0 {
		fmt.Fprintln(os.Stderr, "unexpected argument:", flag.Args()[0])
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	if cfg.DefaultRootInjected {
		log.Printf("config: no [[root]] configured; injecting %s -> %s", cfg.Roots[0].Name, cfg.Roots[0].Path)
	}
	if err := server.Start(context.Background(), cfg); err != nil {
		fmt.Fprintln(os.Stderr, "server error:", err)
		os.Exit(1)
	}
}
