package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"gopkg.in/yaml.v3"
)

func TestCleanroomDownstreamVectors_ConfigContractAndDogfoodIsolation(t *testing.T) {
	useCompiledNomosInPath(t)
	t.Setenv("NOMOS_HOME", t.TempDir())
	repoDir := filepath.Join(t.TempDir(), "nomos-creative")
	seedCleanroomDualPlaneRepo(t, repoDir)
	if err := executeInit(repoDir); err != nil {
		t.Fatalf("executeInit failed: %v", err)
	}

	rawYAMLBytes, err := os.ReadFile(filepath.Join(repoDir, "nomos.yaml"))
	if err != nil {
		t.Fatalf("failed reading scaffolded nomos.yaml: %v", err)
	}
	rawYAML := string(rawYAMLBytes)

	verifyZeroDogfoodLeakageInInit(t, repoDir, rawYAML)
	verifyUncommentedTemplateStrictParity(t, rawYAML)
	verifyDownstreamCreativeDomainSync(t, repoDir)
	verifyLiveConfigGateKnobs(t)
}

func verifyZeroDogfoodLeakageInInit(t *testing.T, repoDir, rawYAML string) {
	t.Helper()
	cfg, err := config.LoadConfig(filepath.Join(repoDir, "nomos.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if len(cfg.Governance.Boundary.ProhibitedCommands) != 0 {
		t.Fatalf("CDV-CFG-01: downstream nomos init leaked internal prohibited_commands: %v", cfg.Governance.Boundary.ProhibitedCommands)
	}
	if len(cfg.Governance.Boundary.ProhibitedPackages) != 0 {
		t.Fatalf("CDV-CFG-01: downstream nomos init leaked internal prohibited_packages: %v", cfg.Governance.Boundary.ProhibitedPackages)
	}
	internalLiterals := []string{".nomos/", ".agent/", ".agents/", ".gemini/", "/workstations/"}
	for _, leak := range internalLiterals {
		if strings.Contains(rawYAML, leak) {
			t.Fatalf("CDV-CFG-01: downstream nomos.yaml leaked internal dogfooding literal %q", leak)
		}
	}
}

func verifyUncommentedTemplateStrictParity(t *testing.T, rawYAML string) {
	t.Helper()
	yamlKeyOrItemRe := regexp.MustCompile(`^(\s*)# (\s*(?:[a-z_]+:|-\s+["'][^"']+["']))(.*)$`)
	var uncommentedLines []string
	for _, line := range strings.Split(rawYAML, "\n") {
		if m := yamlKeyOrItemRe.FindStringSubmatch(line); m != nil {
			uncommentedLines = append(uncommentedLines, m[1]+m[2]+m[3])
			continue
		}
		uncommentedLines = append(uncommentedLines, line)
	}
	fullYAML := strings.Join(uncommentedLines, "\n")

	var strictCfg config.EcosystemConfig
	dec := yaml.NewDecoder(bytes.NewReader([]byte(fullYAML)))
	dec.KnownFields(true)
	if err := dec.Decode(&strictCfg); err != nil {
		t.Fatalf("CDV-CFG-02: uncommenting optional blocks in templates/nomos.yaml failed strict EcosystemConfig decode:\n%v\n--- YAML ---\n%s", err, fullYAML)
	}
	if err := config.ValidateConfig(&strictCfg); err != nil {
		t.Fatalf("CDV-CFG-02: uncommented templates/nomos.yaml failed ValidateConfig: %v", err)
	}
	if strictCfg.Governance.QualityGates.Contracts == nil || !strictCfg.Governance.QualityGates.Contracts.Infra.Enabled {
		t.Fatalf("CDV-CFG-02: expected uncommented contracts.infra block to bind cleanly")
	}
}

func verifyDownstreamCreativeDomainSync(t *testing.T, repoDir string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	wt := startCleanroomTaskInEdit(t, repoDir, "SUB", "Add creative vector portal helper")
	creativePy := "\"\"\"Creative vector portal bridge.\"\"\"\n\ndef resolve_portal_scene() -> str:\n    \"\"\"Return creative scene config path.\"\"\"\n    return \".agents/creative_portal_scene.json\"\n"
	writeWorktreeFile(t, wt, filepath.Join("substrate", "src", "portal.py"), creativePy)
	syncCleanroomTask(t, repoDir, "SUB-1", "Verified downstream domain names and .agents path")

	if _, err := os.Stat(filepath.Join(repoDir, "substrate", "src", "portal.py")); err != nil {
		t.Fatalf("CDV-CFG-01: expected portal.py synced to main in downstream repo: %v", err)
	}
}

func verifyLiveConfigGateKnobs(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "cli"), 0755)

	var body strings.Builder
	body.WriteString("// Package cli tests live nomos.yaml gate thresholds.\npackage cli\n\n")
	body.WriteString("// PortalCmd represents a sample command constructor.\ntype PortalCmd struct{ Use string }\n\n")
	body.WriteString("// BuildPortal builds a command.\nfunc BuildPortal() PortalCmd {\n")
	for i := 0; i < 22; i++ {
		body.WriteString("\t_ = \"line\"\n")
	}
	body.WriteString("\treturn PortalCmd{Use: \"portal\"}\n}\n")
	_ = os.WriteFile(filepath.Join(dir, "cli", "cmd.go"), []byte(body.String()), 0644)

	ctx := &vscope.VerifyContext{RepoRoot: dir, WorktreePath: dir, AllFiles: true}
	tiGate := &verify.ThresholdIntegrityGate{}
	_ = os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte("project: demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: true\n    thresholds:\n      max_file_loc: 600\n"), 0644)
	if res, _ := tiGate.Evaluate(ctx); res.Passed {
		t.Fatalf("CDV-CFG-03: expected strict_mode=true to reject relaxed max_file_loc=600")
	}

	_ = os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte("project: demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n    thresholds:\n      max_file_loc: 15\n"), 0644)
	if res, _ := tiGate.Evaluate(ctx); !res.Passed {
		t.Fatalf("CDV-CFG-03: expected strict_mode=false to permit custom thresholds: %s", res.Message)
	}
	locGate := &verify.MaxFileLOCGate{}
	if res, _ := locGate.Evaluate(ctx); res.Passed {
		t.Fatalf("CDV-CFG-03: expected max_file_loc=15 from nomos.yaml to fail 30-line file")
	}

	_ = os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte("project: demo\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n    prohibited_commands:\n      - \"portal\"\n  quality_gates:\n    forbidden_literals: []\n    forbidden_shims:\n      - \"PortalCmd\"\n"), 0644)
	bGate := &verify.BoundaryGate{}
	if res, _ := bGate.Evaluate(ctx); res.Passed {
		t.Fatalf("CDV-CFG-03: expected prohibited_commands=[portal] to fail BoundaryGate")
	}
	shimGate := &verify.AntiShimGate{}
	if res, _ := shimGate.Evaluate(ctx); res.Passed {
		t.Fatalf("CDV-CFG-03: expected forbidden_shims=[PortalCmd] to fail AntiShimGate")
	}
	litGate := &verify.ForbiddenLiteralsGate{}
	if res, _ := litGate.Evaluate(ctx); !res.Passed {
		t.Fatalf("CDV-CFG-03: expected forbidden_literals=[] to disable literal scanning")
	}
}
