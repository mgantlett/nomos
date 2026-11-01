package main

import (
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
)

func TestMain_RootExecution(t *testing.T) {
	cmd := cli.NewRootCmd()
	if cmd == nil {
		t.Fatalf("expected non-nil root command")
	}
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected root --help to succeed: %v", err)
	}
}
