package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func TestFastFileRadarBudgetAndBreaches(t *testing.T) {
	dir := t.TempDir()
	nomosYAML := "project: radar-test\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: false\n    thresholds:\n      max_file_loc: 20\n      max_func_loc: 15\n      max_cyclomatic_complexity: 10\n      max_cognitive_debt_score: 45\n      min_comment_density_pct: 10\n      min_test_coverage_pct: 55\n"
	if err := os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte(nomosYAML), 0644); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}

	var src strings.Builder
	src.WriteString("package sample\n\n// Compute calculates budget.\nfunc Compute() int {\n")
	for i := 0; i < 12; i++ {
		src.WriteString(fmt.Sprintf("\tx%d := %d\n\t_ = x%d\n", i, i, i))
	}
	// 17 lines total -> 85% of max_file_loc=20
	var compact strings.Builder
	compact.WriteString("package sample\n\n// Compute calculates budget.\nfunc Compute() int {\n")
	for i := 0; i < 12; i++ {
		compact.WriteString(fmt.Sprintf("\tif %d > 0 { return %d }\n", i, i))
	}
	compact.WriteString("\treturn 0\n}\n")

	target := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(target, []byte(compact.String()), 0644); err != nil {
		t.Fatalf("write sample.go: %v", err)
	}

	res, err := EvaluateFastFileRadar(dir, target)
	if err != nil {
		t.Fatalf("EvaluateFastFileRadar failed: %v", err)
	}
	if len(res.Warnings) == 0 && len(res.Breaches) == 0 {
		t.Fatalf("expected >=80%% radar warning or breach, got none (LOC=%d)", res.TotalLOC)
	}
}

func TestModernSecretTokensAndEnvDetection(t *testing.T) {
	dir := t.TempDir()
	_ = workspace.SafeGitCommand(dir, "init", "-b", "main").Run()
	_ = workspace.SafeGitCommand(dir, "config", "user.email", "test@nomos.dev").Run()
	_ = workspace.SafeGitCommand(dir, "config", "user.name", "Nomos Test").Run()

	// Construct modern OpenAI token dynamically so static scanners do not flag literal
	fakeOpenAI := "sk-" + "proj-" + strings.Repeat("A1b2", 8)
	appPy := filepath.Join(dir, "client.py")
	if err := os.WriteFile(appPy, []byte("API_KEY = \""+fakeOpenAI+"\"\n"), 0644); err != nil {
		t.Fatalf("write client.py: %v", err)
	}
	// Also write unignored .env and allowed .env.example
	_ = os.WriteFile(filepath.Join(dir, ".env"), []byte("PORT=8080\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, ".env.example"), []byte("PORT=8080\n"), 0644)

	gate := &SecretGate{}
	res, err := gate.Evaluate(&vscope.VerifyContext{RepoRoot: dir, WorktreePath: dir})
	if err != nil {
		t.Fatalf("SecretGate error: %v", err)
	}
	if res.Passed {
		t.Fatalf("expected SecretGate to fail on OpenAI key and unignored .env")
	}
	if !strings.Contains(res.Message, "OpenAI API key") || !strings.Contains(res.Message, ".env") {
		t.Fatalf("expected both OpenAI key and .env in SecretGate message, got: %s", res.Message)
	}
	if strings.Contains(res.Message, ".env.example") {
		t.Fatalf("expected .env.example to be permitted, got: %s", res.Message)
	}
}

func TestComputeWorkspaceImpactAndDependencyLock(t *testing.T) {
	dir := t.TempDir()
	_ = workspace.SafeGitCommand(dir, "init", "-b", "main").Run()
	_ = workspace.SafeGitCommand(dir, "config", "user.email", "test@nomos.dev").Run()
	_ = workspace.SafeGitCommand(dir, "config", "user.name", "Nomos Test").Run()

	_ = os.MkdirAll(filepath.Join(dir, "pkg", "auth"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "pkg", "api"), 0755)
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"demo"}`), 0644)
	_ = os.WriteFile(filepath.Join(dir, "pkg", "auth", "token.go"), []byte("package auth\n// Verify checks token (@intent: #HandshakeEvent).\nfunc Verify() bool { return true }\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "pkg", "auth", "token_test.go"), []byte("package auth\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "pkg", "api", "handler.go"), []byte("package api\nimport \"example.com/demo/pkg/auth\"\nfunc Run() bool { return auth.Verify() }\n"), 0644)
	_ = workspace.SafeGitCommand(dir, "add", ".").Run()
	_ = workspace.SafeGitCommand(dir, "commit", "-m", "init").Run()

	// Modify token.go and package.json
	_ = os.WriteFile(filepath.Join(dir, "pkg", "auth", "token.go"), []byte("package auth\n// Verify checks token (@intent: #HandshakeEvent).\nfunc Verify() bool { return false }\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"demo","dependencies":{"left-pad":"1.0.0"}}`), 0644)

	impactLines := impact.ComputeWorkspaceImpact(dir, nil)
	if len(impactLines) == 0 {
		t.Fatalf("expected non-empty 1-hop impact summary")
	}
	joined := strings.Join(impactLines, "\n")
	if !strings.Contains(joined, "token_test.go") || !strings.Contains(joined, "importers(1): api") || !strings.Contains(joined, "#HandshakeEvent") {
		t.Fatalf("expected paired test, reverse importer api, and #HandshakeEvent in impact map, got: %s", joined)
	}

	locked := impact.ScanLockedDependencyMutations(&vscope.VerifyContext{RepoRoot: dir, WorktreePath: dir}, dir, true)
	if len(locked) != 1 || !strings.Contains(locked[0], "package.json") {
		t.Fatalf("expected package.json locked dependency violation, got: %v", locked)
	}
}
