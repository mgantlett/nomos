package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/xray"
)

func seedCLIXrayRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "brownfield-svc")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var big strings.Builder
	big.WriteString("package main\n\nfunc EmptyHelper() {}\n\nfunc MainLogic(x int) int {\n")
	for i := 0; i < 410; i++ {
		big.WriteString("\tx++\n")
	}
	big.WriteString("\treturn x\n}\n")
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte(big.String()), 0644); err != nil {
		t.Fatalf("write app.go: %v", err)
	}
	return dir
}

// TestXrayCmd_ReadOnlyJSONAndMarkdownOutputs verifies TC-XRAY-03.
// @verifies TC-XRAY-03
func TestXrayCmd_ReadOnlyJSONAndMarkdownOutputs(t *testing.T) {
	dir := seedCLIXrayRepo(t)

	var jsonBuf bytes.Buffer
	rootJSON := NewRootCmd()
	rootJSON.SetOut(&jsonBuf)
	rootJSON.SetArgs([]string{"xray", dir, "--json"})
	if err := rootJSON.Execute(); err != nil {
		t.Fatalf("nomos xray --json failed: %v", err)
	}
	var rep xray.XrayReport
	if uErr := json.Unmarshal(jsonBuf.Bytes(), &rep); uErr != nil {
		t.Fatalf("invalid xray JSON: %v\n%s", uErr, jsonBuf.String())
	}
	if rep.MonolithFiles < 1 || rep.EmptyStubs < 1 || rep.Initialized {
		t.Fatalf("unexpected XrayReport from CLI: %+v", rep)
	}

	var mdBuf bytes.Buffer
	rootMD := NewRootCmd()
	rootMD.SetOut(&mdBuf)
	rootMD.SetArgs([]string{"xray", dir, "--markdown"})
	if err := rootMD.Execute(); err != nil {
		t.Fatalf("nomos xray --markdown failed: %v", err)
	}
	if !strings.Contains(mdBuf.String(), "## Nomos Brownfield X-Ray: `brownfield-svc`") {
		t.Fatalf("unexpected markdown output:\n%s", mdBuf.String())
	}

	entries, rErr := os.ReadDir(dir)
	if rErr != nil || len(entries) != 1 || entries[0].Name() != "app.go" {
		t.Fatalf("expected zero file mutations in %s after nomos xray, got entries=%v err=%v", dir, entries, rErr)
	}
}

// TestInitCmd_BaselineFlagWritesManifestAndCalibratesRatchet verifies TC-INIT-03.
// @verifies TC-INIT-03
func TestInitCmd_BaselineFlagWritesManifestAndCalibratesRatchet(t *testing.T) {
	t.Setenv(constants.EnvNomosHome, filepath.Join(t.TempDir(), "nomos_home"))
	dir := seedCLIXrayRepo(t)
	runGitOrFail(t, dir, "init", "-b", "main")
	runGitOrFail(t, dir, "config", "user.email", "dev@example.com")
	runGitOrFail(t, dir, "config", "user.name", "Brownfield Maintainer")
	runGitOrFail(t, dir, "add", "app.go")
	runGitOrFail(t, dir, "commit", "-m", "chore: seed legacy monolith")

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"init", "--baseline", "--dir", dir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("nomos init --baseline failed: %v", err)
	}

	manifest, mErr := xray.LoadBaselineManifest(dir)
	if mErr != nil {
		t.Fatalf("expected %s to exist and parse: %v", constants.FileNomosBaselineJSON, mErr)
	}
	if manifest.BaselineEnvelope.ObservedMaxFileLOC <= 400 || manifest.HeadCommit == "" {
		t.Fatalf("expected recorded >400 LOC envelope and git HEAD SHA, got %+v", manifest)
	}

	cfg, cErr := config.LoadConfig(filepath.Join(dir, constants.FileNomosYAML))
	if cErr != nil {
		t.Fatalf("LoadConfig failed: %v", cErr)
	}
	if cfg.Governance.QualityGates.StrictMode {
		t.Fatal("expected nomos init --baseline to enforce strict_mode: false on indebted repo")
	}

	headFiles := runGitOrFail(t, dir, "show", "--name-only", "--pretty=format:", "HEAD")
	if !strings.Contains(headFiles, constants.FileNomosBaselineJSON) {
		t.Fatalf("expected %s committed in baseline governance commit, got:\n%s", constants.FileNomosBaselineJSON, headFiles)
	}
}
