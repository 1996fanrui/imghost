// Command imghost is the user-facing CLI for managing the imghostd daemon.
package main

import (
	"fmt"
	"os"

	"github.com/1996fanrui/imghost/cmd/imghost/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
