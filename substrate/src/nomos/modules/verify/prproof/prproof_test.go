package prproof_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/prproof"
)

func initTestGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustGit(t, repo, "init", "-b", "main")
	mustGit(t, repo, "config", "user.email", "ci@example.com")
	mustGit(t, repo, "config", "user.name", "Nomos CI")
	mustWrite(t, filepath.Join(repo, "nomos.yaml"), "project: demo-pr\ndefault_branch: main\ngovernance:\n  quality_gates:\n    intent_source_roots:\n      - intent\n    substrate_source_roots:\n      - pkg\n")
	mustWrite(t, filepath.Join(repo, "go.mod"), "module example.com/demopr\n\ngo 1.24\n")
	mustWrite(t, filepath.Join(repo, "pkg", "quota", "quota.go"), "package quota\n\n// Allow Quota baseline\nfunc Allow(used, limit int) bool {\n\treturn used < limit\n}\n")
	mustWrite(t, filepath.Join(repo, "pkg", "quota", "quota_test.go"), "package quota\n\nimport \"testing\"\n\nfunc TestAllow(t *testing.T) {\n\tif !Allow(1, 5) || Allow(5, 5) {\n\t\tt.Fatal(\"unexpected\")\n\t}\n}\n")
	mustGit(t, repo, "add", ".")
	mustGit(t, repo, "commit", "-m", "chore: initial main baseline")
	return repo
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	out, err := workspace.RunSafeGit(dir, args...)
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func mustWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

// TestEvaluateDiffMutations_CatchesTautologicalTestsAndPassesBoundaryTests verifies AST micro-mutation probing.
// @verifies TC-MUT-01
// @verifies TC-MUT-02
func TestEvaluateDiffMutations_CatchesTautologicalTestsAndPassesBoundaryTests(t *testing.T) {
	repo := initTestGitRepo(t)
	mustGit(t, repo, "checkout", "-b", "feature/SUB-101")

	mustWrite(t, filepath.Join(repo, "pkg", "quota", "quota.go"),
		"package quota\n\n// Allow Quota baseline\nfunc Allow(used, limit int) bool {\n\treturn used < limit\n}\n\n// CanBurst checks burst eligibility.\nfunc CanBurst(score int) bool {\n\tif score > 10 {\n\t\treturn true\n\t}\n\treturn false\n}\n")
	mustWrite(t, filepath.Join(repo, "pkg", "quota", "quota_test.go"),
		"package quota\n\nimport \"testing\"\n\nfunc TestAllow(t *testing.T) {\n\tif !Allow(1, 5) || Allow(5, 5) {\n\t\tt.Fatal(\"unexpected\")\n\t}\n}\n\nfunc TestCanBurstTautology(t *testing.T) {\n\t_ = CanBurst(20)\n}\n")
	mustGit(t, repo, "add", ".")
	mustGit(t, repo, "commit", "-m", "feat: add CanBurst with tautological test")

	repFail, err := prproof.EvaluateDiffMutations(repo, "main")
	if err != nil {
		t.Fatalf("unexpected EvaluateDiffMutations error: %v", err)
	}
	if repFail.Passed || repFail.MutantsSurvived == 0 || len(repFail.Survivors) == 0 {
		t.Fatalf("expected tautological test to leave surviving mutants, got %+v", repFail)
	}
	summary := prproof.FormatDiffMutationSummary(repFail)
	if !strings.Contains(summary, "[FAIL]") || !strings.Contains(summary, "CanBurst") {
		t.Fatalf("expected failure summary naming CanBurst, got %q", summary)
	}

	mustWrite(t, filepath.Join(repo, "pkg", "quota", "quota_test.go"),
		"package quota\n\nimport \"testing\"\n\nfunc TestAllow(t *testing.T) {\n\tif !Allow(1, 5) || Allow(5, 5) {\n\t\tt.Fatal(\"unexpected\")\n\t}\n}\n\nfunc TestCanBurstBoundary(t *testing.T) {\n\tif !CanBurst(11) {\n\t\tt.Fatal(\"expected 11 to burst\")\n\t}\n\tif CanBurst(10) {\n\t\tt.Fatal(\"expected boundary 10 not to burst\")\n\t}\n\tif CanBurst(3) {\n\t\tt.Fatal(\"expected 3 not to burst\")\n\t}\n}\n")
	mustGit(t, repo, "add", ".")
	mustGit(t, repo, "commit", "-m", "test: add boundary assertions killing all mutants")

	repPass, pErr := prproof.EvaluateDiffMutations(repo, "main")
	if pErr != nil {
		t.Fatalf("unexpected EvaluateDiffMutations error on boundary suite: %v", pErr)
	}
	if !repPass.Passed || repPass.MutantsTotal == 0 || repPass.KillScorePct != 100.0 {
		t.Fatalf("expected 100%% diff mutant kill score, got %+v", repPass)
	}
}

// TestBuildPRProofReceipt_AndMarkdownFormatting verifies SHA-256 sealing, plane classification, and Law 5 scrubbing.
// @verifies TC-PR-01
// @verifies TC-PR-02
func TestBuildPRProofReceipt_AndMarkdownFormatting(t *testing.T) {
	repo := initTestGitRepo(t)
	mustGit(t, repo, "checkout", "-b", "feature/SUB-102")
	mustWrite(t, filepath.Join(repo, "pkg", "quota", "quota.go"),
		"package quota\n\n// Allow Quota baseline\nfunc Allow(used, limit int) bool {\n\treturn used <= limit\n}\n")
	mustWrite(t, filepath.Join(repo, "pkg", "quota", "quota_test.go"),
		"package quota\n\nimport \"testing\"\n\nfunc TestAllow(t *testing.T) {\n\tif !Allow(5, 5) || Allow(6, 5) {\n\t\tt.Fatal(\"unexpected\")\n\t}\n}\n")
	mustGit(t, repo, "add", ".")
	mustGit(t, repo, "commit", "-m", "feat: inclusive quota limit")

	outFile := filepath.Join(t.TempDir(), "pr-proof.md")
	var outBuf bytes.Buffer
	posture := prproof.GatePostureSummary{GatesPassed: 42, GatesTotal: 42, PipelinePass: true}
	passed, err := prproof.RenderVerifyPRAndMutation(&outBuf, repo, posture, prproof.PRProofOptions{
		BaseRef:    "main",
		MutateDiff: true,
		OutputPath: outFile,
	}, false)
	if err != nil {
		t.Fatalf("unexpected RenderVerifyPRAndMutation error: %v", err)
	}
	if !passed {
		t.Fatalf("expected PR proof receipt to pass, output:\n%s", outBuf.String())
	}
	diskBytes, rErr := os.ReadFile(outFile)
	if rErr != nil {
		t.Fatalf("expected --pr-output file %s to exist: %v", outFile, rErr)
	}
	md := string(diskBytes)
	if !strings.Contains(md, "42/42") || !strings.Contains(md, "substrate-only") || !strings.Contains(md, "100.0%") {
		t.Fatalf("unexpected Markdown PR receipt:\n%s", md)
	}
	if home, hErr := os.UserHomeDir(); hErr == nil && home != "" && strings.Contains(md, home) {
		t.Fatalf("Markdown PR receipt leaked host home path %q:\n%s", home, md)
	}

	var jsonBuf bytes.Buffer
	jsonPassed, jErr := prproof.RenderVerifyPRAndMutation(&jsonBuf, repo, posture, prproof.PRProofOptions{BaseRef: "main"}, true)
	if jErr != nil || !jsonPassed || !strings.Contains(jsonBuf.String(), "\"receipt_sha256\"") {
		t.Fatalf("expected JSON PR proof output with receipt_sha256, passed=%v err=%v out=%s", jsonPassed, jErr, jsonBuf.String())
	}
}
