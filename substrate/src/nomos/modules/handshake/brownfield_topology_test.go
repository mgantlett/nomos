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
	mustWrite("sqlglot/helper.py", "def ensure_list(v):\n    return [v]\n")
	mustWrite("sqlglot/dialects/duckdb.py", "class DuckDB:\n    pass\n")
	mustWrite("tests/test_parser.py", "import unittest\n")

	cfg := &config.EcosystemConfig{
		Governance: config.GovernanceConfig{
			QualityGates: config.QualityGatesConfig{
				StrictMode: false,
			},
		},
	}

	topo := handshake.SynthesizeBrownfieldTopology(dir, cfg)
	if topo == nil {
		t.Fatal("expected non-nil BrownfieldTopologyBrief when strict_mode=false")
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
	if len(topo.SharedModules) == 0 || topo.SharedModules[0] != "sqlglot/helper.py" {
		t.Fatalf("expected sqlglot/helper.py in shared modules, got %v", topo.SharedModules)
	}
	if len(topo.Hotspots) == 0 || topo.Hotspots[0].Path != "sqlglot/parser.py" {
		t.Fatalf("expected sqlglot/parser.py hotspot, got %v", topo.Hotspots)
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
