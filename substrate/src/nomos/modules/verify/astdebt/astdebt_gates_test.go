package astdebt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectStructuralClones_GoType2(t *testing.T) {
	fnBody := func(fnName, varName string) string {
		var b strings.Builder
		b.WriteString("package dup\nfunc " + fnName + "(" + varName + " int) int {\n")
		b.WriteString("\tif " + varName + " < 0 {\n\t\treturn 0\n\t}\n")
		for i := 0; i < 14; i++ {
			b.WriteString("\t" + varName + " += 1\n")
		}
		b.WriteString("\treturn " + varName + "\n}\n")
		return b.String()
	}

	files := map[string][]byte{
		"a.go": []byte(fnBody("Alpha", "x")),
		"b.go": []byte(fnBody("Beta", "y")),
	}
	groups := DetectStructuralClones(files, 18)
	if len(groups) != 1 {
		t.Fatalf("expected 1 structural clone group, got %d", len(groups))
	}
	if len(groups[0].Occurrences) != 2 {
		t.Fatalf("expected 2 clone occurrences, got %d", len(groups[0].Occurrences))
	}
}

func TestCheckPolyglotUncheckedErrors(t *testing.T) {
	if got := CheckPolyglotUncheckedErrors(ExtTS, "app.ts", []byte("try { work() } catch (e) {}\n")); len(got) != 1 {
		t.Fatalf("expected 1 TS silent catch violation, got %v", got)
	}
	if got := CheckPolyglotUncheckedErrors(ExtPy, "worker.py", []byte("try:\n    run()\nexcept Exception:\n    pass\n")); len(got) != 1 {
		t.Fatalf("expected 1 Python silent except violation, got %v", got)
	}
	if got := CheckPolyglotUncheckedErrors(ExtRS, "lib.rs", []byte("fn f() {\n    let _ = save_state();\n}\n")); len(got) != 1 {
		t.Fatalf("expected 1 Rust let _ = violation, got %v", got)
	}
	if got := CheckPolyglotUncheckedErrors(ExtRB, "job.rb", []byte("def run\n  work rescue nil\nend\n")); len(got) != 1 {
		t.Fatalf("expected 1 Ruby rescue nil violation, got %v", got)
	}
}

func TestScanCobraSurfaceAndAliases(t *testing.T) {
	dir := t.TempDir()
	good := `package cli
import "github.com/spf13/cobra"
func goodCmd() *cobra.Command {
	return &cobra.Command{Use: "ok", Short: "s", Long: "l", Example: "nomos ok"}
}
`
	bad := `package cli
import "github.com/spf13/cobra"
func badCmd() *cobra.Command {
	return &cobra.Command{Use: "bad", Aliases: []string{"b"}, Short: "s"}
}
`
	if err := os.WriteFile(filepath.Join(dir, "good.go"), []byte(good), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}

	aliases, err := ScanCobraAliasesInDir(dir)
	if err != nil || len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %v (err=%v)", aliases, err)
	}
	violations, err := ScanCobraSurfaceCompletenessInDir(dir)
	if err != nil || len(violations) != 2 {
		t.Fatalf("expected 2 missing Cobra field violations (Long, Example), got %v (err=%v)", violations, err)
	}
}

func TestVerifyNomosSchemaArtifact_DriftDetection(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..", "..", "..", "..")
	schemaBytes, err := os.ReadFile(filepath.Join(repoRoot, "nomos.schema.json"))
	if err != nil {
		t.Fatalf("read nomos.schema.json: %v", err)
	}
	if msg := VerifyNomosSchemaArtifact(repoRoot, schemaBytes); msg != "" {
		t.Fatalf("expected repo nomos.schema.json to match config AST, got: %s", msg)
	}
	if msg := VerifyNomosSchemaArtifact(repoRoot, []byte(`{"$schema":"bad"}`)); msg == "" {
		t.Fatalf("expected invalid schema URI to be rejected")
	}
}
