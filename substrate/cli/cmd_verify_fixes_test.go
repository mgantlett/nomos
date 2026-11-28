package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/autofix"
)

func TestVerifyCmd_FixesAndFixFlags(t *testing.T) {
	dir := seedCLIXrayRepo(t)
	runGitOrFail(t, dir, "init", "-b", "main")
	runGitOrFail(t, dir, "config", "user.email", "dev@example.com")
	runGitOrFail(t, dir, "config", "user.name", "Verify Fixes Tester")
	runGitOrFail(t, dir, "add", "app.go")
	runGitOrFail(t, dir, "commit", "-m", "chore: seed baseline")

	var jsonOut bytes.Buffer
	fixesCmd := NewRootCmd()
	fixesCmd.SetOut(&jsonOut)
	fixesCmd.SetArgs([]string{"verify", "--baseline", "--fixes", "--json", dir})
	if err := fixesCmd.Execute(); err != nil {
		t.Logf("baseline verify --fixes exited non-zero as expected on uninitialized repo: %v", err)
	}

	var plan autofix.VerifyFixPlan
	if err := json.Unmarshal(jsonOut.Bytes(), &plan); err != nil {
		t.Fatalf("expected valid VerifyFixPlan JSON from --fixes --json: %v\n%s", err, jsonOut.String())
	}
	if plan.GatesTotal == 0 {
		t.Fatalf("expected populated gate totals in VerifyFixPlan, got %+v", plan)
	}

	var baseFixErrBuf bytes.Buffer
	baseFixCmd := NewRootCmd()
	baseFixCmd.SetOut(&baseFixErrBuf)
	baseFixCmd.SetArgs([]string{"verify", "--baseline", "--fix", dir})
	err := baseFixCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "protected trunk/baseline") {
		t.Fatalf("expected --fix --baseline to fail closed on protected trunk, got err=%v", err)
	}

	// Verify terminal --fixes output on clean feature branch with an unformatted Go file.
	runGitOrFail(t, dir, "checkout", "-b", "feat/format-check")
	if wErr := os.WriteFile(filepath.Join(dir, "helper.go"), []byte("package main\nfunc   Two()int{return 2}\n"), 0644); wErr != nil {
		t.Fatalf("write helper.go: %v", wErr)
	}
	var termOut bytes.Buffer
	termCmd := NewRootCmd()
	termCmd.SetOut(&termOut)
	termCmd.SetArgs([]string{"verify", "--fixes", dir})
	if execErr := termCmd.Execute(); execErr != nil {
		t.Logf("branch verify --fixes returned expected gate status: %v", execErr)
	}
	if !strings.Contains(termOut.String(), "Nomos Deterministic Repair Plan") {
		t.Fatalf("expected terminal repair plan header, got:\n%s", termOut.String())
	}
}
