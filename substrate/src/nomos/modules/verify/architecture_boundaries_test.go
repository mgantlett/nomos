package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// @verifies TC-OBL-006
// @verifies TC-OBL-007
func TestEvaluateArchitectureBoundariesPolyglotAndFastRadar(t *testing.T) {
	tmp := t.TempDir()

	domainGo := filepath.Join(tmp, "internal", "domain", "order.go")
	routeTS := filepath.Join(tmp, "src", "routes", "page.ts")
	corePy := filepath.Join(tmp, "core", "service.py")
	for _, p := range []string{domainGo, routeTS, corePy} {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
	}

	if err := os.WriteFile(domainGo, []byte("package domain\nimport \"example.com/app/internal/infra/db\"\nvar _ = db.Conn\n"), 0644); err != nil {
		t.Fatalf("write domainGo: %v", err)
	}
	if err := os.WriteFile(routeTS, []byte("import { runQuery } from '../db/client';\nexport const x = runQuery;\n"), 0644); err != nil {
		t.Fatalf("write routeTS: %v", err)
	}
	if err := os.WriteFile(corePy, []byte("from adapters.http_client import send\n"), 0644); err != nil {
		t.Fatalf("write corePy: %v", err)
	}

	rules := []config.ArchitectureBoundaryRule{
		{
			Name:          "domain-isolation",
			From:          "internal/domain/**",
			ForbidImports: []string{"internal/infra/**", "database/sql"},
		},
		{
			Name:      "ui-service-seam",
			From:      "src/routes/**",
			AllowOnly: []string{"$lib/**", "svelte"},
		},
		{
			Name:          "python-core-purity",
			From:          "core/**",
			ForbidImports: []string{"adapters/**"},
		},
	}

	violations := impact.EvaluateArchitectureBoundaries(tmp, nil, rules)
	if len(violations) != 3 {
		t.Fatalf("expected 3 polyglot boundary violations (Go, TS, Python), got %d: %v", len(violations), violations)
	}

	// Verify FastFileRadar (<15ms PreToolUse/PostToolUse hook) catches TS allow_only breach (TC-OBL-007)
	nomosYAML := "project: sample\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: true\n    architecture_boundaries:\n      - name: ui-service-seam\n        from: \"src/routes/**\"\n        allow_only:\n          - \"$lib/**\"\n          - \"svelte\"\n      - name: domain-isolation\n        from: \"internal/domain/**\"\n        forbid_imports:\n          - \"internal/infra/**\"\n    thresholds:\n      max_cognitive_debt_score: 45\n      max_cyclomatic_complexity: 15\n      max_file_loc: 400\n      max_func_loc: 60\n      min_comment_density_pct: 10\n      min_test_coverage_pct: 55\n"
	if err := os.WriteFile(filepath.Join(tmp, "nomos.yaml"), []byte(nomosYAML), 0644); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}

	radarRes, rErr := EvaluateFastFileRadar(tmp, routeTS)
	if rErr != nil {
		t.Fatalf("EvaluateFastFileRadar failed: %v", rErr)
	}
	if len(radarRes.Breaches) == 0 || !strings.Contains(strings.Join(radarRes.Breaches, "; "), "ui-service-seam") {
		t.Fatalf("expected FastFileRadar breach for ../db/client outside allow_only, got: %+v", radarRes.Breaches)
	}

	gate := &IntentTraceabilityGate{}
	gateRes, gErr := gate.Evaluate(&vscope.VerifyContext{RepoRoot: tmp, WorktreePath: tmp})
	if gErr != nil || gateRes == nil || gateRes.Passed {
		t.Fatalf("expected IntentTraceabilityGate to reject boundary violations, got: %+v, err=%v", gateRes, gErr)
	}

	// Fix violations and verify clean pass
	if err := os.WriteFile(domainGo, []byte("package domain\n// Order models a pure entity (@intent nomos_intent_traceability_spec.md).\ntype Order struct{ ID string }\n"), 0644); err != nil {
		t.Fatalf("write clean domainGo: %v", err)
	}
	if err := os.WriteFile(routeTS, []byte("import { api } from '$lib/api';\nexport const x = api;\n"), 0644); err != nil {
		t.Fatalf("write clean routeTS: %v", err)
	}
	if clean := impact.EvaluateArchitectureBoundaries(tmp, []string{domainGo, routeTS}, rules); len(clean) != 0 {
		t.Fatalf("expected 0 violations after refactoring imports, got: %v", clean)
	}
}

// @verifies TC-OBL-008
func TestArchitectureBoundariesDowngradeRejected(t *testing.T) {
	trunkCfg := config.DefaultNomosConfig()
	trunkCfg.Governance.QualityGates.ArchitectureBoundaries = []config.ArchitectureBoundaryRule{
		{Name: "domain-isolation", From: "internal/domain/**", ForbidImports: []string{"internal/infra/**"}},
	}

	wtDeleted := config.DefaultNomosConfig()
	if issue := impact.CheckArchitectureBoundariesDowngrade(trunkCfg, wtDeleted); !strings.Contains(issue, "illegally removes") {
		t.Fatalf("expected boundary rule removal to fail closed, got: %q", issue)
	}

	wtWeakened := config.DefaultNomosConfig()
	wtWeakened.Governance.QualityGates.ArchitectureBoundaries = []config.ArchitectureBoundaryRule{
		{Name: "domain-isolation", From: "internal/domain/**", ForbidImports: []string{"other/**"}},
	}
	if issue := impact.CheckArchitectureBoundariesDowngrade(trunkCfg, wtWeakened); !strings.Contains(issue, "illegally weakens") {
		t.Fatalf("expected boundary rule weakening to fail closed, got: %q", issue)
	}

	if okIssue := impact.CheckArchitectureBoundariesDowngrade(trunkCfg, trunkCfg); okIssue != "" {
		t.Fatalf("expected identical boundary rules to pass, got: %q", okIssue)
	}
}

// @verifies TC-TRC-001
// @verifies TC-TRC-002
// @verifies TC-TRC-003
// @verifies TC-TRC-004
func TestSelfDogfoodTraceabilitySpecObligations(t *testing.T) {
	repoRoot := filepath.Clean("../../../../..")
	if absRoot, aErr := filepath.Abs(repoRoot); aErr == nil {
		repoRoot = absRoot
	}
	if isActiveIntentWorktreeOrBranch(repoRoot) {
		t.Skip("skipping substrate obligation self-dogfooding inside active INT-* worktree (Law 7 causal declaration phase)")
	}
	specPath := filepath.Join("intent", "architecture", "specs", "nomos_intent_traceability_spec.md")
	if _, err := os.Stat(filepath.Join(repoRoot, specPath)); err != nil {
		t.Skipf("workspace spec %s not found: %v", specPath, err)
	}
	if unbound := VerifyExecutableObligations(repoRoot, []string{specPath}); len(unbound) != 0 {
		t.Fatalf("dogfooding failure: unbound TC-* vectors in %s: %v", specPath, unbound)
	}
	subDir := filepath.Join(repoRoot, "substrate")
	if drifted := VerifySpecTypeSignatures(repoRoot, subDir, []string{specPath}); len(drifted) != 0 {
		t.Fatalf("dogfooding failure: struct field drift in %s: %v", specPath, drifted)
	}
}

func isActiveIntentWorktreeOrBranch(repoRoot string) bool {
	if strings.Contains(filepath.ToSlash(repoRoot), "/worktrees/") && strings.Contains(filepath.Base(repoRoot), "INT-") {
		return true
	}
	out, err := workspace.SafeGitCommand(repoRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return false
	}
	branch := strings.TrimSpace(string(out))
	return strings.HasPrefix(branch, "feature/INT-") || strings.HasPrefix(branch, "INT-")
}
