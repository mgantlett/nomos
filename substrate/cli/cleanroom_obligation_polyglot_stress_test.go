package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
)

func seedPolyglotObligationRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "cleanroom@nomos.local"},
		{"config", "user.name", "Nomos Cleanroom"},
	} {
		if out, err := workspace.RunSafeGit(repo, args...); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, string(out))
		}
	}
	for _, rel := range []string{"specs", "src/domain", "src/infra", "app/orders", "tests"} {
		if err := os.MkdirAll(filepath.Join(repo, rel), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
	}
	cfgYAML := "project: polyglot-hex\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: true\n    intent_source_roots:\n      - \"specs\"\n    architecture_boundaries:\n      - name: \"ts-domain-purity\"\n        from: \"src/domain/**\"\n        forbid_imports:\n          - \"src/infra/**\"\n      - name: \"py-orders-purity\"\n        from: \"app/orders/**\"\n        forbid_imports:\n          - \"src/infra/**\"\n    contracts:\n      contracts:\n        enabled: true\n        spec_roots:\n          - \"specs\"\n        strict_types: true\n        verify_obligations: true\n        fail_on_stale_obligations: true\n    thresholds:\n      max_cognitive_debt_score: 45\n      max_cyclomatic_complexity: 15\n      max_file_loc: 400\n      max_func_loc: 60\n      min_comment_density_pct: 10\n      min_test_coverage_pct: 55\n"
	writeCleanroomText(t, filepath.Join(repo, "nomos.yaml"), cfgYAML)
	return repo
}

func writeCleanroomText(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func commitCleanroomRepo(t *testing.T, repo, msg string) string {
	t.Helper()
	if out, err := workspace.RunSafeGit(repo, "add", "."); err != nil {
		t.Fatalf("git add: %v (%s)", err, string(out))
	}
	if out, err := workspace.RunSafeGit(repo, "commit", "-m", msg); err != nil {
		t.Fatalf("git commit: %v (%s)", err, string(out))
	}
	out, err := workspace.RunSafeGit(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func stampCleanroomReceipt(t *testing.T, repo, key, sha, twinKey string, touched []string) {
	t.Helper()
	ts := "2026-11-09T19:00:00Z"
	digests := task.CollectReferencedSpecDigestsWithRoots(repo, touched, twinKey, []string{"specs"})
	err := task.RecordReleaseNote(repo, "", task.ReleaseNotePayload{
		TaskKey: key, CommitSHA: sha, Timestamp: ts, Phase: "SYNC", DoDGatesPassed: 42,
		CapBACTokenID: "cap-" + key, SpecDigests: digests, WalkthroughContent: key + " verified",
		SHA256Receipt: task.ComputeReleaseReceipt(key, sha, ts, "cap-"+key),
	})
	if err != nil {
		t.Fatalf("RecordReleaseNote %s: %v", key, err)
	}
}

func TestCleanroomPolyglotObligationLifecycleStress(t *testing.T) {
	repo := seedPolyglotObligationRepo(t)
	specPath := filepath.Join(repo, "specs", "order_spec.md")
	specV1 := "# Order Spec v1\n\n```typescript\nexport interface OrderDTO {\n  orderId: string;\n  currency: string;\n}\n```\n\n```python\nclass OrderSummary:\n    order_id: str\n    subtotal_cents: int\n```\n\n| Vector | Obligation |\n| :--- | :--- |\n| `TC-ORD-01` | Domain DTO validation |\n| `TC-ORD-02` | Summary calculation |\n"
	writeCleanroomText(t, specPath, specV1)
	commitCleanroomRepo(t, repo, "[INT-1] docs(specs): author order_spec v1")

	t.Run("Stage1To3_FastRadar_DriftBlock_And_V1Receipt", func(t *testing.T) {
		runStage1To3PolyglotV1Lifecycle(t, repo)
	})
	t.Run("Stage4To5_UpstreamSpecTaint_And_Sub3Clearance", func(t *testing.T) {
		runStage4To5PolyglotTaintAndClearance(t, repo, specPath, specV1)
	})
}

func runStage1To3PolyglotV1Lifecycle(t *testing.T, repo string) {
	t.Helper()
	tsDomain := filepath.Join(repo, "src", "domain", "order.ts")
	pyOrders := filepath.Join(repo, "app", "orders", "summary.py")
	pyTest := filepath.Join(repo, "tests", "test_order.py")

	writeCleanroomText(t, tsDomain, "// @intent order_spec.md #OrderDTO\nimport { pool } from '../infra/postgres';\nexport interface OrderDTO {\n  orderId: string;\n}\nexport const dbRef = pool;\n")
	radar, rErr := verify.EvaluateFastFileRadar(repo, tsDomain)
	if rErr != nil || len(radar.Breaches) == 0 || !strings.Contains(strings.Join(radar.Breaches, "; "), "ts-domain-purity") {
		t.Fatalf("Stage 1 failed: expected FastFileRadar breach, got: %+v (err=%v)", radar, rErr)
	}

	writeCleanroomText(t, tsDomain, "// @intent order_spec.md #OrderDTO\nexport interface OrderDTO {\n  orderId: string;\n}\n")
	writeCleanroomText(t, pyOrders, "# @intent order_spec.md #OrderSummary\nclass OrderSummary:\n    order_id: str\n    subtotal_cents: int\n")
	writeCleanroomText(t, pyTest, "# @verifies TC-ORD-01\ndef test_dto():\n    assert True\n")

	gate := &verify.IntentTraceabilityGate{}
	ctx := &verify.VerifyContext{RepoRoot: repo, WorktreePath: repo, TaskID: "SUB-1"}
	if resDrift, err := gate.Evaluate(ctx); err != nil || resDrift == nil || resDrift.Passed || !strings.Contains(resDrift.Message, "currency") {
		t.Fatalf("Stage 2a failed: expected missing TS field 'currency' rejected, got: %+v (err=%v)", resDrift, err)
	}

	writeCleanroomText(t, tsDomain, "// @intent order_spec.md #OrderDTO\nexport interface OrderDTO {\n  orderId: string;\n  currency: string;\n}\n")
	if resUnbound, err := gate.Evaluate(ctx); err != nil || resUnbound == nil || resUnbound.Passed || !strings.Contains(resUnbound.Message, "TC-ORD-02") {
		t.Fatalf("Stage 2b failed: expected unbound TC-ORD-02 rejected, got: %+v (err=%v)", resUnbound, err)
	}

	writeCleanroomText(t, pyTest, "# @verifies TC-ORD-01\n# @verifies TC-ORD-02\ndef test_all():\n    assert True\n")
	if resPass, err := gate.Evaluate(ctx); err != nil || resPass == nil || !resPass.Passed {
		t.Fatalf("Stage 3 failed: expected SUB-1 v1 obligations satisfied, got: %+v (err=%v)", resPass, err)
	}
	shaSub1 := commitCleanroomRepo(t, repo, "[SUB-1] feat(orders): satisfy order_spec v1")
	stampCleanroomReceipt(t, repo, "SUB-1", shaSub1, "INT-1", []string{"src/domain/order.ts", "app/orders/summary.py"})
}

func runStage4To5PolyglotTaintAndClearance(t *testing.T, repo, specPath, specV1 string) {
	t.Helper()
	pyOrders := filepath.Join(repo, "app", "orders", "summary.py")
	pyTest := filepath.Join(repo, "tests", "test_order.py")
	gate := &verify.IntentTraceabilityGate{}

	specV2 := specV1 + "| `TC-ORD-03` | Tax rounding obligation |\n"
	specV2 = strings.Replace(specV2, "    subtotal_cents: int\n", "    subtotal_cents: int\n    tax_cents: int\n", 1)
	writeCleanroomText(t, specPath, specV2)
	commitCleanroomRepo(t, repo, "[INT-2] docs(specs): add tax_cents and TC-ORD-03 to order_spec")

	staleList := resolveStaleObligations(repo)
	if len(staleList) != 1 || staleList[0].SpecPath != "specs/order_spec.md" || staleList[0].LastVerifiedBy != "SUB-1" {
		t.Fatalf("Stage 4a failed: expected specs/order_spec.md flagged stale in status, got: %+v", staleList)
	}
	if resStale, err := gate.Evaluate(&verify.VerifyContext{RepoRoot: repo, WorktreePath: repo, IsBaseline: true}); err != nil || resStale == nil || resStale.Passed {
		t.Fatalf("Stage 4b failed: expected baseline verify after INT-2 mutation to fail closed, got: %+v (err=%v)", resStale, err)
	}

	writeCleanroomText(t, pyOrders, "# @intent order_spec.md #OrderSummary\nclass OrderSummary:\n    order_id: str\n    subtotal_cents: int\n    tax_cents: int\n")
	writeCleanroomText(t, pyTest, "# @verifies TC-ORD-01\n# @verifies TC-ORD-02\n# @verifies TC-ORD-03\ndef test_all_v2():\n    assert True\n")
	if resInFlight, err := gate.Evaluate(&verify.VerifyContext{RepoRoot: repo, WorktreePath: repo, TaskID: "SUB-3"}); err != nil || resInFlight == nil || !resInFlight.Passed {
		t.Fatalf("Stage 5a failed: expected in-flight SUB-3 to pass verify cleanly, got: %+v (err=%v)", resInFlight, err)
	}
	shaSub3 := commitCleanroomRepo(t, repo, "[SUB-3] feat(orders): satisfy order_spec v2 tax obligation")
	stampCleanroomReceipt(t, repo, "SUB-3", shaSub3, "INT-2", []string{"app/orders/summary.py"})

	if postClear := resolveStaleObligations(repo); len(postClear) != 0 {
		t.Fatalf("Stage 5b failed: expected 0 stale obligations after SUB-3 receipt, got: %+v", postClear)
	}
	if resClean, err := gate.Evaluate(&verify.VerifyContext{RepoRoot: repo, WorktreePath: repo, IsBaseline: true}); err != nil || resClean == nil || !resClean.Passed {
		t.Fatalf("Stage 5c failed: expected baseline verify after SUB-3 receipt to pass, got: %+v (err=%v)", resClean, err)
	}
}
