package handshake_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
)

func TestSynthesizeBrownfieldTopology_PolyglotRepo(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		t.Helper()
		dst := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	mustWrite("pyproject.toml", "[project]\nname = \"sqlglot\"\n")
	var bigParser strings.Builder
	for i := 0; i < 360; i++ {
		bigParser.WriteString("def step_" + strings.Repeat("x", 18) + "():\n    return 1\n")
	}
	mustWrite("sqlglot/parser.py", bigParser.String())
	var coreEntry strings.Builder
	for i := 0; i < 140; i++ {
		coreEntry.WriteString("def route_" + strings.Repeat("y", 18) + "():\n    return 2\n")
	}
	mustWrite("sqlglot/core_router.py", coreEntry.String())
	mustWrite("sqlglot/helper.py", "def ensure_list(v):\n    return [v]\n")
	mustWrite("sqlglot/dialects/duckdb.py", "class DuckDB:\n    pass\n")
	mustWrite("tests/test_parser.py", "import unittest\n")

	// Noise paths that must be excluded from SourceRoots, SharedModules, and Core Modules
	mustWrite("_examples/graphql/server.py", bigParser.String())
	mustWrite("typings/index.test-d.ts", strings.Repeat("export type T = string;\n", 200))
	mustWrite("internal/toml-test/runner.py", bigParser.String())
	mustWrite("generated/proto_dump.py", bigParser.String())
	mustWrite("membrane/static/sandbox/assets/three.module-DZxcwB5I.js", strings.Repeat("export const x = 1;\n", 400))
	mustWrite("lib/vendor.min.js", strings.Repeat("function a(){return 1}\n", 400))

	cfg := &config.EcosystemConfig{
		Governance: config.GovernanceConfig{
			QualityGates: config.QualityGatesConfig{
				StrictMode: false,
				Exclude:    []string{"generated"},
			},
		},
	}

	topo := handshake.SynthesizeBrownfieldTopology(dir, cfg)
	if topo == nil {
		t.Fatal("expected non-nil BrownfieldTopologyBrief when strict_mode=false")
	}
	if topo.VerifyScope != config.VerifyScopeDiff {
		t.Fatalf("expected default verify_scope=%s, got %s", config.VerifyScopeDiff, topo.VerifyScope)
	}
	if topo.TDDMode != config.TDDModeRatchet || topo.CoverageMode != config.CoverageModeLive {
		t.Fatalf("unexpected governance modes: tdd=%s cov=%s", topo.TDDMode, topo.CoverageMode)
	}
	if len(topo.Languages) == 0 || topo.Languages[0] != "python" {
		t.Fatalf("expected python primary language, got %v", topo.Languages)
	}
	if len(topo.TestRunners) == 0 || topo.TestRunners[0] != "python3 -m unittest" {
		t.Fatalf("expected unittest runner, got %v", topo.TestRunners)
	}
	for _, root := range topo.SourceRoots {
		if strings.Contains(root, "_examples") || strings.Contains(root, "typings") || strings.Contains(root, "generated") {
			t.Fatalf("unexpected noise directory in SourceRoots: %v", topo.SourceRoots)
		}
	}
	if len(topo.SharedModules) == 0 || topo.SharedModules[0] != "sqlglot/helper.py" {
		t.Fatalf("expected sqlglot/helper.py in shared modules, got %v", topo.SharedModules)
	}
	if len(topo.Hotspots) != 2 || topo.Hotspots[0].Path != "sqlglot/parser.py" || topo.Hotspots[1].Path != "sqlglot/core_router.py" {
		t.Fatalf("expected parser.py and core_router.py (>=120 LOC) without noise dirs, got %+v", topo.Hotspots)
	}

	strictCfg := &config.EcosystemConfig{
		Governance: config.GovernanceConfig{
			QualityGates: config.QualityGatesConfig{
				StrictMode: true,
			},
		},
	}
	if got := handshake.SynthesizeBrownfieldTopology(dir, strictCfg); got != nil {
		t.Fatalf("expected nil brownfield topology in strict_mode=true, got %+v", got)
	}
}

func TestSynthesizeBrownfieldTopology_ActiveBoundariesAndMemorySummary(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src", "domain")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "order.py"), []byte("class Order:\n    pass\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.EcosystemConfig{
		Governance: config.GovernanceConfig{
			QualityGates: config.QualityGatesConfig{
				StrictMode: false,
				ArchitectureBoundaries: []config.ArchitectureBoundaryRule{
					{Name: "domain-purity", From: "src/domain/**", ForbidImports: []string{"src/infra/**"}},
				},
			},
		},
	}
	topo := handshake.SynthesizeBrownfieldTopology(dir, cfg)
	if topo == nil || topo.IntentTwinMode != config.IntentTwinModeOff {
		t.Fatalf("expected non-nil topology with intent_twin_mode=off, got %+v", topo)
	}
	if len(topo.ActiveBoundaries) != 1 || !strings.Contains(topo.ActiveBoundaries[0], "domain-purity") {
		t.Fatalf("expected ActiveBoundaries summary, got %v", topo.ActiveBoundaries)
	}
	memLine := handshake.FormatBrownfieldMemorySummary("orders-svc", topo)
	if !strings.Contains(memLine, "domain-purity") || !strings.Contains(memLine, "stack=[python]") {
		t.Fatalf("unexpected FormatBrownfieldMemorySummary output: %s", memLine)
	}
}
