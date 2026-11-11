package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

func TestHandshakeCmd_JSONOutput(t *testing.T) {
	t.Setenv(constants.EnvNomosNoUpdateCheck, "1")
	cmd := cli.NewHandshakeCmd()
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("NewHandshakeCmd --json failed: %v", err)
	}

	root := cli.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"update", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("update --help failed: %v", err)
	}
	if !json.Valid([]byte(`{"ok":true}`)) || out.Len() == 0 {
		t.Fatalf("expected non-empty update help text")
	}
}

