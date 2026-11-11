package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchTDDExcludeGlobAndConfigHygiene(t *testing.T) {
	cases := []struct {
		pattern string
		relPath string
		want    bool
	}{
		{"**/*.svelte", "apps/dashboard-ui/src/lib/MetricsTab.svelte", true},
		{"**/*.svelte", "MetricsTab.svelte", true},
		{"**/*.svelte", "apps/dashboard-ui/src/lib/trafficOverlay.ts", false},
		{"**/routes/**/+*.ts", "apps/dashboard-ui/src/routes/docs/about/+page.ts", true},
		{"**/routes/**/+*.ts", "apps/dashboard-ui/src/routes/api/traffic/daily-ledger.ts", false},
		{"apps/portal/src/routes", "apps/portal/src/routes/privacy/+page.ts", true},
		{"", "src/index.ts", false},
	}
	for _, tc := range cases {
		if got := MatchTDDExcludeGlob(tc.pattern, tc.relPath); got != tc.want {
			t.Errorf("MatchTDDExcludeGlob(%q, %q) = %v, want %v", tc.pattern, tc.relPath, got, tc.want)
		}
	}

	if isPolyglotImplFileForTDD("vite.config.ts") || isPolyglotImplFileForTDD("svelte.config.js") || isPolyglotImplFileForTDD("app.d.ts") {
		t.Fatalf("expected build config and ambient declaration files to be ignored by TDDGate")
	}
	if !isPolyglotImplFileForTDD("trafficOverlay.ts") {
		t.Fatalf("expected TypeScript logic module to be tracked by TDDGate")
	}
}

func TestWorktreeDiffScopeVsRepoScopeAndTDDExclude(t *testing.T) {
	repo := initTestGitRepo(t)
	strictYAML := "project: acme-web\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: true\n    tdd_exclude:\n      - \"**/*.svelte\"\n      - \"**/routes/**/+*.ts\"\n"
	if err := os.WriteFile(filepath.Join(repo, "nomos.yaml"), []byte(strictYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"name":"acme-web","scripts":{"test":"node --test"}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "vite.config.ts"), []byte("export default {};\n"), 0644); err != nil {
		t.Fatal(err)
	}
	legacyUntestedDir := filepath.Join(repo, "src", "legacy_untested")
	if err := os.MkdirAll(legacyUntestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyUntestedDir, "old.ts"), []byte("export const x = 1;\n"), 0644); err != nil {
		t.Fatal(err)
	}
	routeDir := filepath.Join(repo, "src", "routes", "about")
	if err := os.MkdirAll(routeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(routeDir, "+page.ts"), []byte("export const prerender = true;\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commitAllTestGit(t, repo, "baseline")

	// Simulate an active task worktree editing a tested module + excluded Svelte file.
	featureDir := filepath.Join(repo, "src", "widgets")
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "overlay.ts"), []byte("export function scale(n: number) { return n * 2; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "Panel.svelte"), []byte("<p>ok</p>\n"), 0644); err != nil {
		t.Fatal(err)
	}

	gate := &TDDGate{}
	wtCtx := &VerifyContext{RepoRoot: repo, WorktreePath: repo, TaskID: "SUB-1928"}

	// 1. Untested new logic module in worktree diff MUST fail closed.
	if res, _ := gate.Evaluate(wtCtx); res.Passed {
		t.Fatalf("expected worktree diff TDDGate to fail when overlay.ts lacks paired test")
	}

	// 2. Adding overlay.test.ts satisfies worktree diff scope even though src/legacy_untested exists untouched.
	if err := os.WriteFile(filepath.Join(featureDir, "overlay.test.ts"), []byte("import assert from 'node:assert'; assert.ok(true);\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if res, err := gate.Evaluate(wtCtx); err != nil || !res.Passed {
		t.Fatalf("expected worktree diff TDDGate to pass when touched modules have tests, got err=%v res=%+v", err, res)
	}

	// 3. Full-repo scan (--all or verify_scope=repo) catches untouched src/legacy_untested while ignoring vite.config.ts and excluded +page.ts.
	allRes, _ := gate.Evaluate(&VerifyContext{RepoRoot: repo, WorktreePath: repo, TaskID: "SUB-1928", AllFiles: true})
	if allRes.Passed || !strings.Contains(allRes.Message, "src/legacy_untested") || strings.Contains(allRes.Message, "routes/about") {
		t.Fatalf("expected --all scan to flag only src/legacy_untested and honor tdd_exclude, got %+v", allRes)
	}

	repoScopeYAML := strictYAML + "    verify_scope: repo\n"
	if err := os.WriteFile(filepath.Join(repo, "nomos.yaml"), []byte(repoScopeYAML), 0644); err != nil {
		t.Fatal(err)
	}
	repoScopeRes, _ := gate.Evaluate(wtCtx)
	if repoScopeRes.Passed || !strings.Contains(repoScopeRes.Message, "src/legacy_untested") {
		t.Fatalf("expected verify_scope=repo to enforce full-repo TDD inside worktree, got %+v", repoScopeRes)
	}
}

func TestThresholdIntegrityGuardsVerifyScopeAndTDDExclude(t *testing.T) {
	trunk := t.TempDir()
	wt := t.TempDir()
	trunkYAML := "project: guard\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: true\n    verify_scope: repo\n    tdd_exclude:\n      - \"**/*.svelte\"\n"
	if err := os.WriteFile(filepath.Join(trunk, "nomos.yaml"), []byte(trunkYAML), 0644); err != nil {
		t.Fatal(err)
	}

	downgradeScope := strings.Replace(trunkYAML, "verify_scope: repo", "verify_scope: diff", 1)
	if err := os.WriteFile(filepath.Join(wt, "nomos.yaml"), []byte(downgradeScope), 0644); err != nil {
		t.Fatal(err)
	}
	ti := &ThresholdIntegrityGate{}
	if res, _ := ti.Evaluate(&VerifyContext{RepoRoot: trunk, WorktreePath: wt}); res.Passed {
		t.Fatalf("expected worktree downgrade of verify_scope (repo -> diff) to fail closed")
	}

	expandExclude := trunkYAML + "      - \"**/*.ts\"\n"
	if err := os.WriteFile(filepath.Join(wt, "nomos.yaml"), []byte(expandExclude), 0644); err != nil {
		t.Fatal(err)
	}
	if res, _ := ti.Evaluate(&VerifyContext{RepoRoot: trunk, WorktreePath: wt}); res.Passed {
		t.Fatalf("expected worktree expansion of tdd_exclude to fail closed")
	}
}

