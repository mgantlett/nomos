package xray_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
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
