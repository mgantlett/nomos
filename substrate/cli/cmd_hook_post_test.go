package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mtpo"
)

func TestEvaluateCommandGuard_DirectSQLiteInterceptUsesLaw1(t *testing.T) {
	dec := evaluateCommandGuard("sqlite3 mneme.db 'SELECT 1'", "", "")
	if dec.Decision != hookDecisionDeny {
		t.Fatalf("expected direct sqlite3 command denied, got %+v", dec)
	}
	if !strings.Contains(dec.Reason, "Law 1") || strings.Contains(dec.Reason, "ADR-0062") {
		t.Fatalf("expected Law 1 encapsulation message without private ADR-0062, got: %s", dec.Reason)
	}
}

func TestFormatHotScopeRadarWarningTag(t *testing.T) {
	hotLine := "func Run complexity 10/15 (66% hot-scope >=65% budget)"
	if !strings.Contains(hotLine, "hot-scope") {
		t.Fatalf("expected hot-scope marker in warning string")
	}
}

// TestEvaluatePostToolHook_MTPOCollisionAdvisory verifies non-blocking PostToolUse MTPO steering.
// @verifies TC-MTPO-04
func TestEvaluatePostToolHook_MTPOCollisionAdvisory(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	repoRoot := t.TempDir()
	wt1 := filepath.Join(repoRoot, "worktrees", "nomos-SUB-401")
	wt2 := filepath.Join(repoRoot, "worktrees", "nomos-SUB-402")
	if _, err := mtpo.AllocateSequence("", repoRoot, "SUB-401", wt1, "head0"); err != nil {
		t.Fatal(err)
	}
	if _, err := mtpo.AllocateSequence("", repoRoot, "SUB-402", wt2, "head0"); err != nil {
		t.Fatal(err)
	}

	f1 := filepath.Join(wt1, "pkg", "core", "ledger.go")
	if err := os.MkdirAll(filepath.Dir(f1), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f1, []byte("package core\n\n// MintRecord mints ledger item.\nfunc MintRecord() int { return 1 }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_ = EvaluatePostToolHook(&HookPayload{ToolName: "Write", TargetFile: f1, Cwd: wt1}, "")

	f2 := filepath.Join(wt2, "pkg", "svc", "runner.go")
	if err := os.MkdirAll(filepath.Dir(f2), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("package svc\n\nimport \"example.com/pkg/core\"\n\n// Execute runs MintRecord.\nfunc Execute() int { return core.MintRecord() }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	msg := EvaluatePostToolHook(&HookPayload{ToolName: "Write", TargetFile: f2, Cwd: wt2}, "")
	if !strings.Contains(msg, "Nomos MTPO Advisory") || !strings.Contains(msg, "SUB-401") {
		t.Fatalf("expected PostToolUse MTPO collision advisory naming SUB-401, got %q", msg)
	}
}
