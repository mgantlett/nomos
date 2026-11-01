package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
)

func TestCleanroomDownstreamVectors_LearnDistillationAndTier1Multipliers(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("NOMOS_IN_TOOLCHAIN_GATE", "")
	repoDir := filepath.Join(t.TempDir(), "polyglot-fintech")
	seedCleanroomDualPlaneRepo(t, repoDir)
	if err := executeInit(repoDir); err != nil {
		t.Fatalf("CDV-LRN-00: executeInit failed: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	wtSub1 := verifyOneTurnAtomicChoreAndRadar(t, repoDir)
	verifyPolyglotTrapAndNomosLearnDistillation(t, repoDir, wtSub1)
	verifyHandshakeRecallsDistilledPitfallAndImpact(t, repoDir)
	verifySecretShieldAndDependencyLock(t, repoDir)
}

func verifyOneTurnAtomicChoreAndRadar(t *testing.T, repoDir string) string {
	t.Helper()
	// Mark 1: 1-turn atomic chore inception directly into EDIT phase
	if err := runTaskChoreWithPlane("Add Python ledger reconciliation module", "substrate"); err != nil {
		t.Fatalf("CDV-LRN-01: runTaskChoreWithPlane failed: %v", err)
	}
	wt := filepath.Join(repoDir, "worktrees", "polyglot-fintech-SUB-1")
	if fi, err := os.Stat(wt); err != nil || !fi.IsDir() {
		t.Fatalf("CDV-LRN-01: expected SUB-1 worktree provisioned at %s: %v", wt, err)
	}

	ev, err := handshake.SynthesizeDynamicHandshake(wt)
	if err != nil {
		t.Fatalf("CDV-LRN-01: SynthesizeDynamicHandshake failed: %v", err)
	}
	if string(ev.Phase) != "EDIT" || ev.ActiveTask != "SUB-1" {
		t.Fatalf("CDV-LRN-01: expected 1-turn chore in EDIT phase for SUB-1, got phase=%s task=%s", ev.Phase, ev.ActiveTask)
	}

	// Mark 2: Sub-15ms single-file AST radar check on worktree file
	pyTarget := filepath.Join(wt, "substrate", "src", "reconcile.py")
	_ = os.MkdirAll(filepath.Dir(pyTarget), 0755)
	cleanPy := "\"\"\"Ledger reconciliation.\"\"\"\n\ndef reconcile_ledger(delta: int) -> int:\n    \"\"\"Return reconciled delta.\"\"\"\n    return delta\n"
	if err := os.WriteFile(pyTarget, []byte(cleanPy), 0644); err != nil {
		t.Fatalf("write reconcile.py: %v", err)
	}
	radarRes, rErr := verify.EvaluateFastFileRadar(repoDir, pyTarget)
	if rErr != nil || len(radarRes.Breaches) > 0 {
		t.Fatalf("CDV-LRN-02: expected clean fast-file radar on valid Python file, got err=%v breaches=%v", rErr, radarRes.Breaches)
	}
	return wt
}

func verifyPolyglotTrapAndNomosLearnDistillation(t *testing.T, repoDir, wt string) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed in test environment")
	}

	// Mark 3: Inject broken Python syntax and prove GoToolchainGate polyglot extension catches it
	brokenPy := "\"\"\"Broken reconciliation syntax.\"\"\"\n\ndef reconcile_ledger(delta: int) -> int\n    return delta\n"
	pyTarget := filepath.Join(wt, "substrate", "src", "reconcile.py")
	if err := os.WriteFile(pyTarget, []byte(brokenPy), 0644); err != nil {
		t.Fatalf("write broken reconcile.py: %v", err)
	}

	// Running gate check records the gate trap into transient task buffer
	_, gateErr := executeGateCheckForTask("SUB-1")
	if gateErr == nil || !strings.Contains(gateErr.Error(), "Python syntax compilation failed") {
		t.Fatalf("CDV-LRN-03: expected polyglot Python syntax error to fail toolchain gate, got: %v", gateErr)
	}

	// Fix syntax error + add consumer module for 1-hop impact testing
	fixedPy := "\"\"\"Verified reconciliation module.\"\"\"\n\ndef reconcile_ledger(delta: int) -> int:\n    \"\"\"Return non-negative delta.\"\"\"\n    return delta if delta >= 0 else 0\n"
	consumerPy := "\"\"\"Settlement consumer.\"\"\"\nfrom reconcile import reconcile_ledger\n\ndef settle(amount: int) -> int:\n    \"\"\"Settle amount.\"\"\"\n    return reconcile_ledger(amount)\n"
	_ = os.WriteFile(pyTarget, []byte(fixedPy), 0644)
	_ = os.WriteFile(filepath.Join(wt, "substrate", "src", "settlement.py"), []byte(consumerPy), 0644)

	// Mark 5: Sync task -> automatically distills Gate Trap -> Verified Fix into local mneme.db
	syncCleanroomTask(t, repoDir, "SUB-1", "Added missing colon on def reconcile_ledger signature and verified Python bytecode compilation")
}

func verifyHandshakeRecallsDistilledPitfallAndImpact(t *testing.T, repoDir string) {
	t.Helper()
	// Start SUB-2 via 1-turn chore and modify reconcile.py
	if err := runTaskChoreWithPlane("Extend reconciliation tolerance", "substrate"); err != nil {
		t.Fatalf("CDV-LRN-04: start SUB-2 chore failed: %v", err)
	}
	wt2 := filepath.Join(repoDir, "worktrees", "polyglot-fintech-SUB-2")
	pyTarget := filepath.Join(wt2, "substrate", "src", "reconcile.py")
	updatedPy := "\"\"\"Verified reconciliation module v2.\"\"\"\n\ndef reconcile_ledger(delta: int) -> int:\n    \"\"\"Return clamped delta.\"\"\"\n    return max(0, delta)\n"
	_ = os.WriteFile(pyTarget, []byte(updatedPy), 0644)

	// Handshake inside SUB-2 must surface LearnedPitfalls, VerifiedPatterns, AND ImpactSummary!
	ev, err := handshake.SynthesizeDynamicHandshake(wt2)
	if err != nil {
		t.Fatalf("CDV-LRN-04: SynthesizeDynamicHandshake in SUB-2 failed: %v", err)
	}
	if len(ev.LearnedPitfalls) == 0 || !strings.Contains(ev.LearnedPitfalls[0], "Python syntax compilation failed") {
		t.Fatalf("CDV-LRN-04: expected handshake to recall distilled SUB-1 Python pitfall, got: %v", ev.LearnedPitfalls)
	}
	if len(ev.VerifiedPatterns) == 0 || !strings.Contains(ev.VerifiedPatterns[0], "reconcile_ledger") {
		t.Fatalf("TV-LRN-07: expected handshake to recall positive SUB-1 verified pattern blueprint, got: %v", ev.VerifiedPatterns)
	}
	if len(ev.ImpactSummary) == 0 || !strings.Contains(strings.Join(ev.ImpactSummary, "\n"), "1-hop dependents: 1") {
		t.Fatalf("CDV-LRN-04: expected handshake 1-hop impact map to detect settlement.py dependent, got: %v", ev.ImpactSummary)
	}

	// TV-LRN-08: Statutory write provenance guard rejects manual CLI '--kind pattern' and '--kind pitfall'
	for _, forbiddenKind := range []string{"pattern", "pitfall"} {
		memCmd := NewMemCmd()
		memCmd.SetArgs([]string{"save", "spoofed unverified entry", "--kind", forbiddenKind})
		if mErr := memCmd.Execute(); mErr == nil || !strings.Contains(mErr.Error(), "statutory") {
			t.Fatalf("TV-LRN-08: expected manual 'nomos mem save --kind %s' to fail closed, got: %v", forbiddenKind, mErr)
		}
	}

	syncCleanroomTask(t, repoDir, "SUB-2", "Clamped reconciliation delta cleanly")

	// TV-LRN-09: After >= 3 recalls of the statutory pitfall, GraduationCandidates surfaces Tier A->B promotion
	_, _ = handshake.SynthesizeDynamicHandshake(repoDir)
	evGrad, gErr := handshake.SynthesizeDynamicHandshake(repoDir)
	if gErr != nil || len(evGrad.GraduationCandidates) == 0 {
		t.Fatalf("TV-LRN-09: expected recurring statutory memory to surface in GraduationCandidates, got err=%v cands=%v", gErr, evGrad.GraduationCandidates)
	}
}

func verifySecretShieldAndDependencyLock(t *testing.T, repoDir string) {
	t.Helper()
	// Enable lock_dependencies via an INT-1 chore!
	if err := runTaskChoreWithPlane("Enable dependency lockfile protection", "intent"); err != nil {
		t.Fatalf("CDV-LRN-05: intent chore failed: %v", err)
	}
	wtInt := filepath.Join(repoDir, "worktrees", "polyglot-fintech-INT-1")
	nomosCfgPath := filepath.Join(wtInt, "nomos.yaml")
	rawCfg, _ := os.ReadFile(nomosCfgPath)
	lockedCfg := strings.Replace(string(rawCfg), "enforce: true", "enforce: true\n        lock_dependencies: true", 1)
	_ = os.WriteFile(nomosCfgPath, []byte(lockedCfg), 0644)
	syncCleanroomTask(t, repoDir, "INT-1", "Enabled governance.boundary.lock_dependencies in nomos.yaml")

	// Start SUB-3 and attempt to leak a GitHub token + mutate requirements.txt
	if err := runTaskChoreWithPlane("Test secret and lockfile shield", "substrate"); err != nil {
		t.Fatalf("CDV-LRN-06: SUB-3 chore failed: %v", err)
	}
	wtSub3 := filepath.Join(repoDir, "worktrees", "polyglot-fintech-SUB-3")
	fakeGH := "ghp_" + strings.Repeat("Z9x8Y7w6", 5)
	_ = os.WriteFile(filepath.Join(wtSub3, "substrate", "src", "leak.py"), []byte("TOKEN = \""+fakeGH+"\"\n"), 0644)
	_ = os.WriteFile(filepath.Join(wtSub3, "requirements.txt"), []byte("requests==2.32.0\n"), 0644)

	_, gateErr := executeGateCheckForTask("SUB-3")
	if gateErr == nil {
		t.Fatalf("CDV-LRN-06: expected SecretGate and BoundaryGate lock_dependencies to reject SUB-3")
	}
	errText := gateErr.Error()
	if !strings.Contains(errText, "GitHub token") || !strings.Contains(errText, "lock_dependencies") {
		t.Fatalf("CDV-LRN-06: expected both GitHub token and lock_dependencies violations, got: %s", errText)
	}

	mgr, _ := workspace.NewManager(repoDir)
	_ = mgr.RemoveWorktree("SUB-3")
}
