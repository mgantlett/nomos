// Package main provides the canonical zero-alias CLI entrypoint for the Nomos Gen 4 substrate.
package main

import (
	"fmt"
	"os"

	"github.com/mgantlett/nomos/substrate/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
