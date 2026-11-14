package xray_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/xray"
)

func seedPolyglotHangoverFixture(t *testing.T, root string) {
	t.Helper()
	mustWrite := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	cloneBody := buildRepeatedGoClone("ProcessOrderAlpha")
	clonePeer := buildRepeatedGoClone("ProcessOrderBeta")
	mustWrite("src/domain/alpha.go", cloneBody+"\nfunc EmptyStub() {}\n")
	mustWrite("src/domain/beta.go", clonePeer+"\n")

	var monolith strings.Builder
	monolith.WriteString("def legacy_router(db, user_id):\n")
	monolith.WriteString("    db.execute(f\"SELECT * FROM accounts WHERE id = '{user_id}'\")\n")
	for i := 0; i < 410; i++ {
		monolith.WriteString("    x = 1\n")
	}
	monolith.WriteString("    raise NotImplementedError\n")
	mustWrite("src/infra/legacy_db.py", monolith.String())

	mustWrite("src/domain/alpha_test.go", "package domain\nimport \"testing\"\nfunc TestSkipped(t *testing.T) {\n\tt.Skip(\"later\")\n}\nfunc TestGuardedOS(t *testing.T) {\n\tif false {\n\t\tt.Skip(\"windows only\")\n\t}\n}\n")
	mustWrite("static/assets/vendor-chunk.js", "function a(){return 1}"+strings.Repeat(";if(x){y++}", 400)+"\n")
}

func buildRepeatedGoClone(fnName string) string {
	var sb strings.Builder
	sb.WriteString("package domain\n\n// " + fnName + " processes an item.\nfunc " + fnName + "(items []int) int {\n\ttotal := 0\n")
	for i := 0; i < 20; i++ {
		sb.WriteString("\tif len(items) > 0 {\n\t\ttotal += items[0]\n\t}\n")
	}
	sb.WriteString("\treturn total\n}\n")
	return sb.String()
}

// TestScanRepository_PolyglotHangoverAndReadOnlyGuarantee verifies TC-XRAY-01.
// @verifies TC-XRAY-01
func TestScanRepository_PolyglotHangoverAndReadOnlyGuarantee(t *testing.T) {
	root := filepath.Join(t.TempDir(), "legacy-hangover")
	seedPolyglotHangoverFixture(t, root)

	rep, err := xray.ScanRepository(root, xray.DefaultScanOptions())
	if err != nil {
		t.Fatalf("ScanRepository failed: %v", err)
	}
	if rep.Initialized {
		t.Fatal("expected uninitialized repository to report Initialized=false")
	}
	if rep.ProjectName != "legacy-hangover" || rep.FilesScanned != 4 {
		t.Fatalf("unexpected footprint: %+v", rep)
	}
	if rep.CloneGroups < 1 || rep.MonolithFiles < 1 || rep.OversizedFuncs < 1 {
		t.Fatalf("expected clone, monolith, and oversized function detection, got %+v", rep)
	}
	if rep.EmptyStubs < 1 || rep.SkippedTests != 1 || rep.SecuritySinks < 1 {
		t.Fatalf("expected stub, exactly 1 unconditional skipped test, and CWE sink detection, got %+v", rep)
	}
	if rep.HangoverScore <= 0 || rep.Grade == "A" || len(rep.Hotspots) == 0 {
		t.Fatalf("expected elevated hangover score and hotspots, got %+v", rep)
	}
	if rep.BaselineEnvelope.ObservedMaxFileLOC <= 400 || rep.NextCommand != "nomos init --baseline" {
		t.Fatalf("unexpected BaselineEnvelope or NextCommand: %+v", rep)
	}
	if _, statErr := os.Stat(filepath.Join(root, constants.FileNomosYAML)); !os.IsNotExist(statErr) {
		t.Fatal("expected ScanRepository not to scaffold nomos.yaml")
	}
	if _, statErr := os.Stat(filepath.Join(root, constants.FileNomosBaselineJSON)); !os.IsNotExist(statErr) {
		t.Fatal("expected ScanRepository not to write nomos.baseline.json")
	}
}

// TestXrayFormatting_PathScrubbingAndBaselineRoundTrip verifies TC-XRAY-02.
// @verifies TC-XRAY-02
func TestXrayFormatting_PathScrubbingAndBaselineRoundTrip(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "scrub-target")
	seedPolyglotHangoverFixture(t, root)

	rep, err := xray.ScanRepository(root, xray.XrayScanOptions{TopHotspots: 2})
	if err != nil {
		t.Fatalf("ScanRepository failed: %v", err)
	}
	rawJSON, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	md := xray.FormatMarkdown(rep)
	term := xray.FormatTerminal(rep)

	for label, payload := range map[string]string{"json": string(rawJSON), "markdown": md, "terminal": term} {
		if strings.Contains(payload, parent) {
			t.Fatalf("expected %s output to scrub parent host path %q, got:\n%s", label, parent, payload)
		}
	}
	if !strings.Contains(md, "## Nomos Brownfield X-Ray: `scrub-target`") || !strings.Contains(term, "AI Hangover Score") {
		t.Fatalf("unexpected formatted output:\nMD:\n%s\nTERM:\n%s", md, term)
	}

	writtenPath, wErr := xray.WriteBaselineManifest(root, rep)
	if wErr != nil || filepath.Base(writtenPath) != constants.FileNomosBaselineJSON {
		t.Fatalf("WriteBaselineManifest failed: path=%s err=%v", writtenPath, wErr)
	}
	loaded, lErr := xray.LoadBaselineManifest(root)
	if lErr != nil {
		t.Fatalf("LoadBaselineManifest failed: %v", lErr)
	}
	if loaded.SchemaVersion != xray.BaselineSchemaVersion || loaded.HangoverScore != rep.HangoverScore || loaded.BaselineEnvelope.ObservedMaxFileLOC != rep.BaselineEnvelope.ObservedMaxFileLOC {
		t.Fatalf("loaded manifest mismatch: got %+v want score=%d", loaded, rep.HangoverScore)
	}
}

func TestScanRepository_DayZeroVerifyParitySensorsAndHashedBundles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "parity-target")
	writeFixture := func(rel, body string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	var magic strings.Builder
	magic.WriteString("package svc\n\nfunc run() error { return nil }\n")
	magic.WriteString("func Overloaded(a, b, c, d, e, f, g, h, i int) int {\n\t_ = run()\n")
	magic.WriteString("\tif a > 0 {\n\t\tfor j := 0; j < b; j++ {\n\t\t\tif c > 0 {\n\t\t\t\tswitch d {\n\t\t\t\tcase 1:\n\t\t\t\t\tif e > 0 {\n\t\t\t\t\t\tfor k := 0; k < f; k++ {\n\t\t\t\t\t\t\treturn k\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n")
	magic.WriteString("\tconst hostCfg = \"/" + "Users" + "/dev/app.yaml\"\n\t_ = hostCfg\n\t_ = []string{\n")
	for idx := 0; idx < 12; idx++ {
		magic.WriteString("\t\t\"REPEATED_MAGIC_LITERAL\",\n")
	}
	magic.WriteString("\t}\n\treturn i\n}\n")
	writeFixture("src/svc/handler.go", magic.String())
	writeFixture("static/sovereign-sandbox/assets/gitGraphDiagram-WNU8YQGX-C4AM6nWs.js", "export function x() {\n  return 1;\n}\n")

	rep, err := xray.ScanRepository(root, xray.DefaultScanOptions())
	if err != nil {
		t.Fatalf("ScanRepository failed: %v", err)
	}
	if rep.FilesScanned != 1 {
		t.Fatalf("expected hashed bundle chunk to be excluded (FilesScanned=1), got %d", rep.FilesScanned)
	}
	if rep.HighArityFuncs != 1 || rep.DeepNestingFuncs != 1 || rep.UncheckedErrors != 1 || rep.SecretPathLeaks != 1 || rep.MagicStringGroups != 1 {
		t.Fatalf("expected all 5 parity counters to equal 1, got %+v", rep)
	}
	if rep.BaselineEnvelope.ObservedMaxParams != 9 || rep.BaselineEnvelope.ObservedMaxNesting != 6 {
		t.Fatalf("expected ObservedMaxParams=9 ObservedMaxNesting=6, got %+v", rep.BaselineEnvelope)
	}
	if len(rep.Hotspots) != 1 || rep.Hotspots[0].UncheckedErrors != 1 || rep.Hotspots[0].SecretLeaks != 1 {
		t.Fatalf("unexpected hotspot payload: %+v", rep.Hotspots)
	}
}

func TestScanRepository_HonorsGitIgnoreAndBuildNext(t *testing.T) {
	root := filepath.Join(t.TempDir(), "git-governed-app")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("custom_artifacts/\n**/build.next/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "clean.go"), []byte("package src\n\n// Add sums two integers.\nfunc Add(a, b int) int { return a + b }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ignoredDir := filepath.Join(root, "custom_artifacts")
	buildNextDir := filepath.Join(root, "ui", "build.next", "client", "_app", "immutable", "chunks")
	if err := os.MkdirAll(ignoredDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(buildNextDir, 0755); err != nil {
		t.Fatal(err)
	}
	noisyChunk := "export function bad() { try { run(); } catch (e) {} }\n" + strings.Repeat("// filler\n", 420)
	if err := os.WriteFile(filepath.Join(ignoredDir, "generated.ts"), []byte(noisyChunk), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(buildNextDir, "BuZu7gLC.js"), []byte(noisyChunk), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := workspace.RunSafeGit(root, "init"); err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	if _, err := workspace.RunSafeGit(root, "add", ".gitignore", "src/clean.go"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	rep, err := xray.ScanRepository(root, xray.DefaultScanOptions())
	if err != nil {
		t.Fatalf("ScanRepository failed: %v", err)
	}
	if rep.FilesScanned != 1 || rep.HangoverScore != 0 || len(rep.Hotspots) != 0 {
		t.Fatalf("expected only tracked clean.go (score 0, 1 file), got %+v", rep)
	}
}
