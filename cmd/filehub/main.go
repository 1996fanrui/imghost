// Command filehub is the user-facing CLI for managing the filehubd daemon.
package main

import (
	"fmt"
	"os"

	"github.com/1996fanrui/filehub/cmd/filehub/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
