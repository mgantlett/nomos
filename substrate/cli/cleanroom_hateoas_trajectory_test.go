package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
)

func TestCleanroomDownstreamVectors_ZeroSourceHATEOASTrajectory(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	t.Setenv("NOMOS_IN_TOOLCHAIN_GATE", "")

	repoDir := filepath.Join(t.TempDir(), "aurora-commerce")
	seedZeroSourceBrownfieldRepo(t, repoDir)
	if err := executeInit(repoDir); err != nil {
		t.Fatalf("CDV-HAT-01: executeInit failed: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	verifyBrownfieldMarkerParityAndCleanTemplate(t, repoDir)
	verifyAllPhasesDirectiveResolutionAndZeroLeaks(t)
	verifyTurnByTurnHATEOASTrajectoryAndTmpGuard(t, repoDir)
	verifyCapabilitiesDrillDownAndGateRemedies(t, repoDir)
}

func seedZeroSourceBrownfieldRepo(t *testing.T, repoDir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repoDir, "substrate", "src"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_ = workspace.SafeGitCommand(repoDir, "init", "-b", "main").Run()
	_ = workspace.SafeGitCommand(repoDir, "config", "user.email", "eng@aurora.example").Run()
	_ = workspace.SafeGitCommand(repoDir, "config", "user.name", "Aurora Eng").Run()

	py := "\"\"\"Checkout totals.\"\"\"\n\ndef total_cents(items: list[int]) -> int:\n    \"\"\"Sum item cents.\"\"\"\n    return sum(items)\n"
	writeRawFile(t, filepath.Join(repoDir, "substrate", "src", "checkout.py"), py)
	writeRawFile(t, filepath.Join(repoDir, "AGENTS.md"), "# Aurora Commerce Rules\n- Enforce integer cent arithmetic\n")
	writeRawFile(t, filepath.Join(repoDir, "CLAUDE.md"), "# Aurora Claude Rules\n- Keep handlers pure and deterministic\n")

	_ = workspace.SafeGitCommand(repoDir, "add", ".").Run()
	_ = workspace.SafeGitCommand(repoDir, "commit", "-m", "Initial Aurora brownfield commit").Run()
}

func verifyBrownfieldMarkerParityAndCleanTemplate(t *testing.T, repoDir string) {
	t.Helper()
	agentsRaw, err := os.ReadFile(filepath.Join(repoDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("CDV-HAT-01: read AGENTS.md: %v", err)
	}
	agents := string(agentsRaw)
	if !strings.Contains(agents, "# Aurora Commerce Rules") {
		t.Fatalf("CDV-HAT-01: brownfield AGENTS.md custom rules lost during merge")
	}
	for _, want := range []string{"nomos directive", "nomos capabilities", "nomos task chore"} {
		if !strings.Contains(agents, want) {
			t.Fatalf("CDV-HAT-01: expected AGENTS.md to surface %q", want)
		}
	}
	for _, leak := range []string{"worktrees/nomos-", "intent/architecture/specs", "cmd/nomos", "Maintain and advance the `nomos`"} {
		if strings.Contains(agents, leak) {
			t.Fatalf("CDV-HAT-01: downstream AGENTS.md leaked Foundry-internal string %q", leak)
		}
	}
	claudeRaw, _ := os.ReadFile(filepath.Join(repoDir, "CLAUDE.md"))
	if !strings.Contains(string(claudeRaw), "# Aurora Claude Rules") {
		t.Fatalf("CDV-HAT-01: brownfield CLAUDE.md custom rules lost during merge")
	}
	if err := runVerification(repoDir, "", true, true, false); err != nil {
		t.Fatalf("CDV-HAT-01: brownfield repo with custom suffix outside protocol markers failed 42-gate verification: %v", err)
	}
}

func verifyAllPhasesDirectiveResolutionAndZeroLeaks(t *testing.T) {
	t.Helper()
	for _, phase := range types.AllPhases {
		names := handshake.LoadPhaseDirectives("", phase)
		if len(names) == 0 {
			t.Fatalf("CDV-HAT-02: phase %s returned zero directives", phase)
		}
		for _, name := range names {
			if name == "phase_idle_discipline" {
				t.Fatalf("CDV-HAT-02: phantom directive %q still emitted for phase %s", name, phase)
			}
			body, err := handshake.GetEmbeddedDirective(name)
			if err != nil || strings.TrimSpace(string(body)) == "" {
				t.Fatalf("CDV-HAT-02: phase %s directive %q failed embedded lookup: %v", phase, name, err)
			}
		}
	}

	all, lErr := handshake.ListEmbeddedDirectives()
	if lErr != nil || len(all) < 16 {
		t.Fatalf("CDV-HAT-02: expected at least 16 embedded Markdown directives, got %d (err=%v)", len(all), lErr)
	}
	for _, d := range all {
		if !strings.HasSuffix(d, ".md") {
			t.Fatalf("CDV-HAT-02: non-Markdown asset leaked into embedded directive catalog: %s", d)
		}
		body, err := handshake.GetEmbeddedDirective(d)
		if err != nil {
			t.Fatalf("CDV-HAT-02: failed reading %s: %v", d, err)
		}
		for _, leak := range []string{"worktrees/nomos-", "intent/architecture/specs", "substrate/src/", "cmd/nomos", "Sophia", "gsi-sov", "gsi-open", "vault-intent"} {
			if strings.Contains(string(body), leak) {
				t.Fatalf("CDV-HAT-02: embedded directive %s leaked dogfood or foundry lore %q", d, leak)
			}
		}
	}
}

func verifyTurnByTurnHATEOASTrajectoryAndTmpGuard(t *testing.T, repoDir string) {
	t.Helper()
	assertIdleHandshakeAndStatusAffordances(t)

	choreOut := captureStdoutRun(t, func() error {
		return runTaskChoreWithPlane("Implement checkout tax rounding", "substrate")
	})
	wt := filepath.Join(repoDir, "worktrees", "aurora-commerce-SUB-1")
	wantWalk := filepath.Join(wt, ".tmp", "walkthrough-SUB-1.md")
	if !strings.Contains(choreOut, "Next Actions:") || !strings.Contains(choreOut, "nomos task sync SUB-1 -F "+wantWalk) {
		t.Fatalf("CDV-HAT-03: task chore missing HATEOAS Next Actions:\n%s", choreOut)
	}

	verifyTmpScratchGuardExemption(t, repoDir)
	executeWorktreeEditCommitAndSyncTrajectory(t, repoDir, wt)
}

func assertIdleHandshakeAndStatusAffordances(t *testing.T) {
	t.Helper()
	hsJSON := captureStdoutRun(t, func() error {
		cmd := NewHandshakeCmd()
		cmd.SetArgs([]string{"--json"})
		return cmd.Execute()
	})
	var ev handshake.HandshakeEvent
	if err := json.Unmarshal([]byte(hsJSON), &ev); err != nil {
		t.Fatalf("CDV-HAT-03: unmarshal handshake JSON: %v", err)
	}
	if ev.Phase != types.PhaseIdle || len(ev.NextActions) == 0 {
		t.Fatalf("CDV-HAT-03: expected IDLE handshake with non-empty next_actions, got phase=%s actions=%d", ev.Phase, len(ev.NextActions))
	}

	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"status", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("CDV-HAT-03: status --json failed: %v", err)
	}
	var st StatusReport
	if err := json.Unmarshal(buf.Bytes(), &st); err != nil || len(st.NextActions) == 0 {
		t.Fatalf("CDV-HAT-03: expected status --json with non-empty next_actions, err=%v", err)
	}
}

func verifyTmpScratchGuardExemption(t *testing.T, repoDir string) {
	t.Helper()
	wtGuard := filepath.Join(repoDir, ".agents", "scripts", "worktree-guard.sh")
	tmpWalk := filepath.Join(repoDir, ".tmp", "walkthrough-SUB-1.md")
	trunkSrc := filepath.Join(repoDir, "substrate", "src", "checkout.py")

	allowDec := invokeHookScript(t, wtGuard, repoDir, map[string]any{
		"toolCall": map[string]any{"name": "write_file", "args": map[string]any{"Path": tmpWalk}},
	})
	if allowDec.Decision != "allow" {
		t.Fatalf("CDV-HAT-04: expected worktree-guard.sh to allow .tmp walkthrough %s, got %+v", tmpWalk, allowDec)
	}
	denyDec := invokeHookScript(t, wtGuard, repoDir, map[string]any{
		"toolCall": map[string]any{"name": "write_file", "args": map[string]any{"Path": trunkSrc}},
	})
	if denyDec.Decision != "deny" {
		t.Fatalf("CDV-HAT-04: expected worktree-guard.sh to deny trunk source edit, got %+v", denyDec)
	}
}

func executeWorktreeEditCommitAndSyncTrajectory(t *testing.T, repoDir, wt string) {
	t.Helper()
	taxPy := "\"\"\"Tax rounding.\"\"\"\n\ndef compute_tax_cents(subtotal: int, bps: int) -> int:\n    \"\"\"Compute rounded tax cents.\"\"\"\n    return (subtotal * bps + 5000) // 10000\n"
	writeWorktreeFile(t, wt, filepath.Join("substrate", "src", "tax.py"), taxPy)

	if err := os.Chdir(wt); err != nil {
		t.Fatalf("chdir worktree: %v", err)
	}
	commitOut := captureStdoutRun(t, func() error {
		cmd := newTaskCommitCmd()
		cmd.SetArgs([]string{"SUB-1", "-m", "Add checkout tax rounding"})
		return cmd.Execute()
	})
	if !strings.Contains(commitOut, "Next Actions:") || !strings.Contains(commitOut, "nomos task sync SUB-1 -F") {
		t.Fatalf("CDV-HAT-03: task commit missing Next Actions:\n%s", commitOut)
	}
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir repoDir: %v", err)
	}

	syncNoFlag := newTaskSyncCmd()
	syncNoFlag.SetArgs([]string{"SUB-1"})
	syncErr := syncNoFlag.Execute()
	var nErr *types.NomosError
	if !errors.As(syncErr, &nErr) || !strings.Contains(nErr.Remedy, ".tmp/walkthrough-SUB-1.md") {
		t.Fatalf("CDV-HAT-04: expected task sync without -F to return .tmp/walkthrough-SUB-1.md remedy, got err=%v", syncErr)
	}

	walk := writeWalkthroughTemp(t, repoDir, "SUB-1", "Implemented deterministic basis-point tax rounding")
	syncOut := captureStdoutRun(t, func() error {
		cmd := newTaskSyncCmd()
		cmd.SetArgs([]string{"SUB-1", "-F", walk})
		return cmd.Execute()
	})
	if !strings.Contains(syncOut, "Next Actions:") || !strings.Contains(syncOut, "nomos handshake") {
		t.Fatalf("CDV-HAT-03: task sync missing Next Actions:\n%s", syncOut)
	}
}

func verifyCapabilitiesDrillDownAndGateRemedies(t *testing.T, repoDir string) {
	t.Helper()
	for _, target := range []string{"cell_7_1", "plane_separation"} {
		root := NewRootCmd()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetArgs([]string{"capabilities", target, "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("CDV-HAT-05: capabilities %s --json failed: %v", target, err)
		}
		var detail CapabilityDetail
		if err := json.Unmarshal(buf.Bytes(), &detail); err != nil || detail.ID != target || detail.Remedy == "" {
			t.Fatalf("CDV-HAT-05: unexpected capability detail for %s: %+v (err=%v)", target, detail, err)
		}
	}

	unknownCmd := NewRootCmd()
	unknownCmd.SetOut(new(bytes.Buffer))
	unknownCmd.SetArgs([]string{"capabilities", "nonexistent_probe_999"})
	if uErr := unknownCmd.Execute(); uErr == nil {
		t.Fatalf("CDV-HAT-05: expected unknown capability ID to fail closed")
	}

	assertDownstreamSafeRemedies(t, repoDir)
}

func assertDownstreamSafeRemedies(t *testing.T, repoDir string) {
	t.Helper()
	forbidden := []string{"stash", "Milestone", "Wazero", "intent/contracts/cue/"}
	for _, cell := range verify.DefaultBaselineCells() {
		for _, bad := range forbidden {
			if strings.Contains(cell.Remedy, bad) {
				t.Fatalf("CDV-HAT-06: cell %s remedy contains forbidden string %q: %s", cell.ID, bad, cell.Remedy)
			}
		}
	}

	dirtyPath := filepath.Join(repoDir, "substrate", "src", "untracked_drift.py")
	writeRawFile(t, dirtyPath, "x = 1\n")
	defer func() { _ = os.Remove(dirtyPath) }()

	driftRes, err := (&verify.EcosystemDriftGate{}).Evaluate(&verify.VerifyContext{RepoRoot: repoDir, IsBaseline: true})
	if err != nil || driftRes.Passed {
		t.Fatalf("CDV-HAT-06: expected EcosystemDriftGate to detect untracked trunk drift, err=%v", err)
	}
	if strings.Contains(strings.ToLower(driftRes.Remedy), "stash") || !strings.Contains(driftRes.Remedy, "nomos task chore") {
		t.Fatalf("CDV-HAT-06: EcosystemDriftGate remedy must avoid git stash and suggest nomos task chore, got: %s", driftRes.Remedy)
	}
}

func captureStdoutRun(t *testing.T, fn func() error) string {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	runErr := fn()
	_ = w.Close()
	os.Stdout = origStdout

	out, rErr := io.ReadAll(r)
	_ = r.Close()
	if rErr != nil {
		t.Fatalf("read captured stdout: %v", rErr)
	}
	if runErr != nil {
		t.Fatalf("captured command failed: %v\nstdout:\n%s", runErr, string(out))
	}
	return string(out)
}
