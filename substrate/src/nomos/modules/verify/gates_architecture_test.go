package verify

import (
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestArchitectureGatesRegistration(t *testing.T) {
	for _, id := range []string{"boundary", "ecosystem_drift", "zero_alias", "magic_strings", "magic_string_duplication"} {
		g := GateByID(id)
		if g == nil {
			t.Fatalf("expected gate %s to be registered", id)
		}
	}
}

func TestBoundaryGateClean(t *testing.T) {
	g := &BoundaryGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected boundary gate to pass: %s", res.Message)
	}
}

func TestZeroAliasGateClean(t *testing.T) {
	g := &ZeroAliasGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected zero alias gate to pass: %s", res.Message)
	}
}

func TestMagicStringsGateClean(t *testing.T) {
	g := &MagicStringsGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected magic strings gate to pass: %s", res.Message)
	}
}

func TestCollectGoStringLiterals_SkipsImportsAndStructTags(t *testing.T) {
	dir := t.TempDir()
	src := `package sample
import (
	"regexp"
	"time"
)
type Item struct {
	Status string ` + "`" + `json:"status"` + "`" + `
}
var re = regexp.MustCompile("hello_world")
var ts = time.RFC3339
`
	file := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(file, []byte(src), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	counts := make(map[string]map[string]int)
	collectGoStringLiterals(token.NewFileSet(), file, counts)
	if counts["sample"]["regexp"] != 0 || counts["sample"]["time"] != 0 {
		t.Fatalf("expected import path literals to be excluded, got %+v", counts["sample"])
	}
	if counts["sample"]["hello_world"] != 1 {
		t.Fatalf("expected regular string literal to be counted once, got %+v", counts["sample"])
	}
}

func TestScanCobraSurfaceCompletenessInDir(t *testing.T) {
	dir := t.TempDir()
	stubSrc := `package cli
import "github.com/spf13/cobra"
func bad() *cobra.Command {
	return &cobra.Command{Use: "bad", Short: "missing long and example"}
}
`
	if err := os.WriteFile(filepath.Join(dir, "cmd_bad.go"), []byte(stubSrc), 0644); err != nil {
		t.Fatalf("write stub failed: %v", err)
	}
	violations, err := ScanCobraSurfaceCompletenessInDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) != 2 {
		t.Fatalf("expected 2 missing field violations (Long, Example), got %d: %v", len(violations), violations)
	}
}
