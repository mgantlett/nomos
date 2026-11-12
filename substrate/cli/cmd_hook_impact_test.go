package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
)

func TestHookSymlinkSecretAndFirstEditImpact(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	ws := t.TempDir()

	// 1. ln -s blocked by Law 6
	decSym := EvaluatePreToolHook(&HookPayload{ToolName: "Bash", CommandLine: "ln -s ../solution_cache/a.json out.json", Cwd: ws}, ws)
	if decSym.Decision != hookDecisionDeny || !strings.Contains(decSym.Reason, "ZeroFilesystemAliasGate") {
		t.Fatalf("expected symlink command block, got: %+v", decSym)
	}

	// 2. printenv SECRET blocked by Law 7
	decEnv := EvaluatePreToolHook(&HookPayload{ToolName: "Bash", CommandLine: "printenv WEBHOOK_SECRET_KEY > log.txt", Cwd: ws}, ws)
	if decEnv.Decision != hookDecisionDeny || !strings.Contains(decEnv.Reason, "Secret Airlock") {
		t.Fatalf("expected printenv secret dump block, got: %+v", decEnv)
	}

	// 3. Read outside hermeticDir blocked
	decRead := EvaluatePreToolHook(&HookPayload{ToolName: "Read", TargetFile: "../verifier/vectors.json", Cwd: ws}, ws)
	if decRead.Decision != hookDecisionDeny {
		t.Fatalf("expected read escape outside hermeticDir to be denied, got: %+v", decRead)
	}

	// 4. First-edit 1-hop impact fires once per file and suppresses repeat spam
	wtDir := filepath.Join(ws, "worktrees", "demo-SUB-1")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, "mux.go"), []byte("package chi\ntype Router interface { Get() }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	treePath := filepath.Join(wtDir, "tree.go")
	if err := os.WriteFile(treePath, []byte("package chi\nfunc Register() int { return 1 }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p := &HookPayload{ToolName: "Write", TargetFile: treePath, Cwd: wtDir}
	adv1 := EvaluatePostToolHook(p, wtDir)
	if !strings.Contains(adv1, "Nomos 1-Hop Impact") || !strings.Contains(adv1, "mux.go") {
		t.Fatalf("expected first-edit 1-hop impact naming mux.go, got: %q", adv1)
	}
	adv2 := EvaluatePostToolHook(p, wtDir)
	if strings.Contains(adv2, "Nomos 1-Hop Impact") {
		t.Fatalf("expected second edit on same file not to repeat 1-hop impact, got: %q", adv2)
	}

	// 5. Compact handshake smoke check
	t.Setenv("NOMOS_ADVISORY_STYLE", "terse")
	renderHandshakeBanner(&handshake.HandshakeEvent{
		Repository: "demo",
		Phase:      "EDIT",
		ActiveTask: "SUB-1",
		GateCount:  42,
		BrownfieldTopology: &handshake.BrownfieldTopologyBrief{
			Languages: []string{"go"},
		},
	}, false, ws)
}
