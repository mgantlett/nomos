package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
)

func TestMain_RootExecution(t *testing.T) {
	var stderr bytes.Buffer
	code := run(func() error {
		cmd := cli.NewRootCmd()
		if cmd == nil {
			t.Fatalf("expected non-nil root command")
		}
		cmd.SetArgs([]string{"--help"})
		return cmd.Execute()
	}, &stderr)
	if code != 0 {
		t.Fatalf("expected root --help exit code 0, got %d (stderr=%q)", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr on --help, got %q", stderr.String())
	}
}

func TestMain_ErrorExitAndFormatting(t *testing.T) {
	var stderr bytes.Buffer
	code := run(func() error {
		cmd := cli.NewRootCmd()
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		cmd.SetArgs([]string{"--nonexistent-flag"})
		return cmd.Execute()
	}, &stderr)
	if code != 1 {
		t.Fatalf("expected error exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown flag") {
		t.Fatalf("expected formatted CLI error on stderr, got %q", stderr.String())
	}
}
