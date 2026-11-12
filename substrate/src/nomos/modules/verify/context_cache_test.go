package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/polyrun"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func TestPipelineMemoizesConfigAndScopeAcrossGates(t *testing.T) {
	ctx := &vscope.VerifyContext{RepoRoot: t.TempDir(), TaskID: "SUB-99"}
	cfgLoads := 0
	scopeLoads := 0

	vscope.BeginVerifyContextCache(ctx)
	for i := 0; i < 5; i++ {
		cfg := vscope.MemoizedVerifyConfig(ctx, func() *config.EcosystemConfig {
			cfgLoads++
			return config.DefaultNomosConfig()
		})
		if cfg == nil {
			t.Fatalf("expected non-nil memoized config")
		}
		files := vscope.MemoizedScopeFiles(ctx, ctx.RepoRoot, func() []string {
			scopeLoads++
			// Nested memoized lookup must not deadlock on non-reentrant mutex.
			nestedCfg := vscope.MemoizedVerifyConfig(ctx, func() *config.EcosystemConfig {
				cfgLoads++
				return config.DefaultNomosConfig()
			})
			if nestedCfg == nil {
				t.Fatalf("expected nested config")
			}
			return []string{"pkg/a.go"}
		})
		if len(files) != 1 || files[0] != "pkg/a.go" {
			t.Fatalf("unexpected memoized scope files: %v", files)
		}
	}
	vscope.EndVerifyContextCache(ctx)

	if cfgLoads != 1 || scopeLoads != 1 {
		t.Fatalf("expected 1 config load and 1 scope load inside pipeline cache, got cfg=%d scope=%d", cfgLoads, scopeLoads)
	}

	// After eviction, subsequent calls must evaluate fresh state.
	vscope.MemoizedScopeFiles(ctx, ctx.RepoRoot, func() []string {
		scopeLoads++
		return []string{"pkg/b.go"}
	})
	if scopeLoads != 2 {
		t.Fatalf("expected uncached evaluation after endVerifyContextCache, got scopeLoads=%d", scopeLoads)
	}
}

func TestImpactMapEmitsDependentPathsAndExactGoModule(t *testing.T) {
	dir := t.TempDir()
	mustMkdir := func(rel string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, rel), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
	}
	mustWrite := func(rel, body string) {
		t.Helper()
		mustMkdir(filepath.Dir(rel))
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	mustWrite("go.mod", "module example.com/myrepo\n\ngo 1.22\n")
	mustWrite("pkg/config/cfg.go", "package config\nfunc Load() int { return 1 }\n")
	mustWrite("pkg/config/cfg_test.go", "package config\n")
	mustWrite("cmd/app/main.go", "package main\nimport \"example.com/myrepo/pkg/config\"\nfunc main() { _ = config.Load() }\n")
	mustWrite("cmd/other/main.go", "package main\nimport \"thirdparty.dev/vendor/pkg/config\"\nfunc main() { _ = config.Load() }\n")
	mustWrite("web/store.ts", "export const state = 1;\n")
	mustWrite("web/view.ts", "import { state } from './store';\nexport const v = state;\n")

	impacts := impact.ComputeWorkspaceImpact(dir, []string{"pkg/config/cfg.go", "web/store.ts"})
	joined := strings.Join(impacts, "\n")
	if !strings.Contains(joined, "importers(1): app [cmd/app]") {
		t.Fatalf("expected exact module importer cmd/app without third-party suffix collision, got:\n%s", joined)
	}
	if strings.Contains(joined, "other") {
		t.Fatalf("expected thirdparty.dev/vendor/pkg/config not to collide with example.com/myrepo/pkg/config, got:\n%s", joined)
	}
	if !strings.Contains(joined, "1-hop dependents: 1 file(s) [web/view.ts]") {
		t.Fatalf("expected polyglot dependent path [web/view.ts] in impact map, got:\n%s", joined)
	}
}

func TestTruncateToolchainOutput_PreservesCompilerErrorBeforeFailSummary(t *testing.T) {
	okPrefix := strings.Repeat("ok  \tgithub.com/mgantlett/nomos/substrate/pkg/alpha\t0.014s\tcoverage: 84.2% of statements\n", 25)
	buildErr := "# github.com/mgantlett/nomos/substrate/pkg/broken\npkg/broken/runner.go:55:6: helperFn redeclared in this block\nFAIL\tgithub.com/mgantlett/nomos/substrate/pkg/broken [build failed]"
	got := polyrun.TruncateToolchainOutput(okPrefix + buildErr)
	if !strings.Contains(got, "pkg/broken/runner.go:55:6: helperFn redeclared in this block") {
		t.Fatalf("expected compiler error line preserved before FAIL summary, got %q", got)
	}
}

func TestCollectTaskCommitPolyglotFiles_ExactTaskBoundary(t *testing.T) {
	dir := initGitTestRepo(t)
	runGit := func(args ...string) {
		t.Helper()
		if out, err := workspace.SafeGitCommand(dir, args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
	}

	otherFile := filepath.Join(dir, "substrate", "other.go")
	if err := os.WriteFile(otherFile, []byte("package testmod\nfunc Other() int { return 20 }\n"), 0644); err != nil {
		t.Fatalf("write other.go: %v", err)
	}
	runGit("add", "-A")
	runGit("commit", "-m", "[SUB-20] add Other")

	modified := make(map[string]bool)
	vscope.CollectTaskCommitPolyglotFiles(dir, "SUB-2", modified)
	if len(modified) != 0 {
		t.Fatalf("expected SUB-2 log grep not to match [SUB-20] commit, got: %v", modified)
	}

	vscope.CollectTaskCommitPolyglotFiles(dir, "SUB-20", modified)
	if !modified[otherFile] {
		t.Fatalf("expected SUB-20 log grep to match [SUB-20] commit, got: %v", modified)
	}
}
