// Package main provides the canonical zero-alias CLI entrypoint for the Nomos Gen 4 substrate.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mgantlett/nomos/substrate/cli"
)

func main() {
	os.Exit(run(cli.Execute, os.Stderr))
}

func run(execFn func() error, stderr io.Writer) int {
	if err := execFn(); err != nil {
		fmt.Fprint(stderr, cli.FormatCLIError(err))
		return 1
	}
	return 0
}
