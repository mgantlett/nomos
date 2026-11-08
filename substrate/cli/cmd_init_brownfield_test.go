package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func TestExecuteInit_GreenfieldPreservesStrictMode(t *testing.T) {
	dir := t.TempDir()
	_ = workspace.SafeGitCommand(dir, "init", "-b", "main").Run()
	if err := executeInit(dir); err != nil {
		t.Fatalf("executeInit failed on greenfield repo: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "nomos.yaml"))
	if err != nil {
		t.Fatalf("read nomos.yaml: %v", err)
	}
	if !strings.Contains(string(raw), "strict_mode: true") {
		t.Fatalf("expected greenfield repo to keep strict_mode: true, got:\n%s", string(raw))
	}
}

func TestExecuteInit_BrownfieldLegacyGodFileAndVerify(t *testing.T) {
	dir := t.TempDir()
	_ = workspace.SafeGitCommand(dir, "init", "-b", "main").Run()
	_ = workspace.SafeGitCommand(dir, "config", "user.email", "dev@nomos.local").Run()
	_ = workspace.SafeGitCommand(dir, "config", "user.name", "Brownfield Tester").Run()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/legacyrepo\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	pkgDir := filepath.Join(dir, "pkg", "engine")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("mkdir pkg/engine: %v", err)
	}

	var legacy strings.Builder
	legacy.WriteString("package engine\n\nfunc LegacyUndocumentedGod(code int) int {\n")
	for i := 1; i <= 22; i++ {
		legacy.WriteString(fmt.Sprintf("\tif code == %d {\n\t\treturn %d\n\t}\n", i, i*10))
	}
	legacy.WriteString("\treturn 0\n}\n\n")
	for i := 0; i < 90; i++ {
		legacy.WriteString(fmt.Sprintf("func helperPad%d(v int) int {\n\t// padding line 1\n\t// padding line 2\n\t// padding line 3\n\treturn v + %d\n}\n", i, i))
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "legacy.go"), []byte(legacy.String()), 0644); err != nil {
		t.Fatalf("write legacy.go: %v", err)
	}
	testSrc := "package engine\n\nimport \"testing\"\n\nfunc TestLegacySmoke(t *testing.T) {\n\tif LegacyUndocumentedGod(1) != 10 {\n\t\tt.Fatal(\"unexpected\")\n\t}\n}\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "legacy_test.go"), []byte(testSrc), 0644); err != nil {
		t.Fatalf("write legacy_test.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("# Changelog\nSee [removed doc](docs/removed_2018.md).\n"), 0644); err != nil {
		t.Fatalf("write CHANGELOG.md: %v", err)
	}

	_ = workspace.SafeGitCommand(dir, "add", ".").Run()
	_ = workspace.SafeGitCommand(dir, "commit", "-m", "chore: legacy baseline").Run()

	if err := executeInit(dir); err != nil {
		t.Fatalf("executeInit failed on legacy brownfield repo: %v", err)
	}

	rawCfg, err := os.ReadFile(filepath.Join(dir, "nomos.yaml"))
	if err != nil {
		t.Fatalf("read nomos.yaml: %v", err)
	}
	cfgStr := string(rawCfg)
	if !strings.Contains(cfgStr, "strict_mode: false") {
		t.Fatalf("expected brownfield nomos.yaml to set strict_mode: false, got:\n%s", cfgStr)
	}
	if !strings.Contains(cfgStr, "- \"pkg\"") {
		t.Fatalf("expected brownfield nomos.yaml to auto-detect substrate_source_roots pkg, got:\n%s", cfgStr)
	}

	if err := runVerification(dir, "", true, false, false); err != nil {
		t.Fatalf("expected brownfield repo with 500+ LOC God file and <20%% coverage to pass nomos verify after init, got: %v", err)
	}
}
