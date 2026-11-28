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

	headAfterDefault := strings.TrimSpace(runGitOrFail(t, dir, "rev-parse", "HEAD"))
	headBefore := strings.TrimSpace(runGitOrFail(t, dir, "rev-list", "-n1", "HEAD"))
	if headAfterDefault != headBefore {
		t.Fatalf("expected %s hash check consistent", headAfterDefault)
	}
	logCount := strings.TrimSpace(runGitOrFail(t, dir, "rev-list", "--count", "HEAD"))
	if logCount != "1" {
		t.Fatalf("expected default nomos init --baseline NOT to silently commit on main, got %s commits", logCount)
	}

	wtDir := seedCLIXrayRepo(t)
	runGitOrFail(t, wtDir, "init", "-b", "main")
	runGitOrFail(t, wtDir, "config", "user.email", "dev@example.com")
	runGitOrFail(t, wtDir, "config", "user.name", "Brownfield Maintainer")
	runGitOrFail(t, wtDir, "add", "app.go")
	runGitOrFail(t, wtDir, "commit", "-m", "chore: seed legacy monolith")

	wtCmd := NewRootCmd()
	wtCmd.SetArgs([]string{"init", "--baseline", "--worktrees", "--dir", wtDir})
	if err := wtCmd.Execute(); err != nil {
		t.Fatalf("nomos init --baseline --worktrees failed: %v", err)
	}
	headFiles := runGitOrFail(t, wtDir, "show", "--name-only", "--pretty=format:", "HEAD")
	if !strings.Contains(headFiles, constants.FileNomosBaselineJSON) {
		t.Fatalf("expected %s committed in baseline governance commit with --worktrees, got:\n%s", constants.FileNomosBaselineJSON, headFiles)
	}
}

func TestXraySarifAndInitCIFlagIntegration(t *testing.T) {
	// @verifies TC-CI-01
	// @verifies TC-SARIF-01
	dir := seedCLIXrayRepo(t)
	runGitOrFail(t, dir, "init", "-b", "main")
	runGitOrFail(t, dir, "config", "user.email", "dev@example.com")
	runGitOrFail(t, dir, "config", "user.name", "CI Verifier")
	runGitOrFail(t, dir, "add", "app.go")
	runGitOrFail(t, dir, "commit", "-m", "chore: seed monolith")

	var out strings.Builder
	xrayCmd := NewRootCmd()
	xrayCmd.SetOut(&out)
	xrayCmd.SetArgs([]string{"xray", dir, "--sarif"})
	if err := xrayCmd.Execute(); err != nil {
		t.Fatalf("nomos xray --sarif failed: %v", err)
	}
	if !strings.Contains(out.String(), `"version": "2.1.0"`) || !strings.Contains(out.String(), "NOMOS-XRAY-HOTSPOT") {
		t.Fatalf("unexpected xray --sarif output:\n%s", out.String())
	}

	conflictCmd := NewRootCmd()
	conflictCmd.SetArgs([]string{"init", "--ci", "--local", "--dir", dir})
	if err := conflictCmd.Execute(); err == nil {
		t.Fatal("expected nomos init --ci --local to fail closed")
	}

	var initOut strings.Builder
	ciCmd := NewRootCmd()
	ciCmd.SetOut(&initOut)
	ciCmd.SetArgs([]string{"init", "--ci", "--dir", dir})
	if err := ciCmd.Execute(); err != nil {
		t.Fatalf("nomos init --ci failed: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".github", "workflows", "nomos-verify.yml")); statErr != nil {
		t.Fatalf("expected offline GitHub Actions workflow written: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".gitlab-ci.yml")); statErr != nil {
		t.Fatalf("expected offline GitLab CI workflow written: %v", statErr)
	}

	var vOut strings.Builder
	vCmd := NewRootCmd()
	vCmd.SetOut(&vOut)
	vCmd.SetArgs([]string{"verify", "--baseline", "--sarif", dir})
	if err := vCmd.Execute(); err == nil {
		t.Fatal("expected verify --sarif on uncommitted trunk artifacts to return non-zero while emitting SARIF")
	}
	if !strings.Contains(vOut.String(), `"version": "2.1.0"`) || !strings.Contains(vOut.String(), `"name": "Nomos"`) || !strings.Contains(vOut.String(), `"level": "error"`) {
		t.Fatalf("unexpected verify --sarif output:\n%s", vOut.String())
	}
}

func TestXrayCmd_DiffAndBadgeFlags(t *testing.T) {
	dir := seedCLIXrayRepo(t)
	runGitOrFail(t, dir, "init", "-b", "main")
	runGitOrFail(t, dir, "config", "user.email", "dev@example.com")
	runGitOrFail(t, dir, "config", "user.name", "Xray Verifier")
	runGitOrFail(t, dir, "add", "app.go")
	runGitOrFail(t, dir, "commit", "-m", "chore: seed legacy monolith")
	runGitOrFail(t, dir, "checkout", "-b", "feat/diff-check")
	if err := os.WriteFile(filepath.Join(dir, "clean.go"), []byte("package main\n\nfunc Ok(v int) int { return v + 1 }\n"), 0644); err != nil {
		t.Fatalf("write clean.go: %v", err)
	}

	var diffBuf bytes.Buffer
	diffCmd := NewRootCmd()
	diffCmd.SetOut(&diffBuf)
	diffCmd.SetArgs([]string{"xray", dir, "--diff", "main", "--json"})
	if err := diffCmd.Execute(); err != nil {
		t.Fatalf("nomos xray --diff main --json failed: %v", err)
	}
	var diffRep xray.XrayReport
	if err := json.Unmarshal(diffBuf.Bytes(), &diffRep); err != nil {
		t.Fatalf("unmarshal diff report: %v", err)
	}
	if diffRep.DiffBaseRef != "main" || diffRep.FilesScanned != 1 || diffRep.MonolithFiles != 0 {
		t.Fatalf("expected clean diff scan of 1 file, got %+v", diffRep)
	}

	var badgeBuf bytes.Buffer
	badgeCmd := NewRootCmd()
	badgeCmd.SetOut(&badgeBuf)
	badgeCmd.SetArgs([]string{"xray", dir, "--diff", "main", "--badge"})
	if err := badgeCmd.Execute(); err != nil {
		t.Fatalf("nomos xray --badge failed: %v", err)
	}
	if !strings.Contains(badgeBuf.String(), "https://img.shields.io/badge/Nomos_X--Ray-") || !strings.Contains(badgeBuf.String(), "https://sophialabs.ca/xray") {
		t.Fatalf("unexpected --badge output:\n%s", badgeBuf.String())
	}

	var svgBuf bytes.Buffer
	svgCmd := NewRootCmd()
	svgCmd.SetOut(&svgBuf)
	svgCmd.SetArgs([]string{"xray", dir, "--badge-svg"})
	if err := svgCmd.Execute(); err != nil {
		t.Fatalf("nomos xray --badge-svg failed: %v", err)
	}
	if !strings.Contains(svgBuf.String(), "<svg ") || !strings.Contains(svgBuf.String(), "Nomos X-Ray") {
		t.Fatalf("unexpected --badge-svg output:\n%s", svgBuf.String())
	}

	conflictCmd := NewRootCmd()
	conflictCmd.SetArgs([]string{"xray", dir, "--badge", "--badge-svg"})
	if err := conflictCmd.Execute(); err == nil {
		t.Fatal("expected combining --badge and --badge-svg to fail closed")
	}
}
