package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
)

func TestDetectCollision_Green(t *testing.T) {
	hullA := &cli.WorktreeHull{
		TaskKey:        "SUB-1191",
		WorktreePath:   "/tmp/worktrees/nomos-SUB-1191",
		TouchedFiles:   []string{"core/types/errors.go"},
		MutatedExports: []string{"NewNomosError"},
		TotalMass:      12.5,
		MeanEntropy:    3.2,
	}
	hullB := &cli.WorktreeHull{
		TaskKey:        "SUB-1192",
		WorktreePath:   "/tmp/worktrees/nomos-SUB-1192",
		TouchedFiles:   []string{"core/config/paths.go"},
		MutatedExports: []string{"ResolveWorkstationRoot"},
		TotalMass:      15.0,
		MeanEntropy:    2.8,
	}

	collision := cli.DetectCollision(hullA, hullB)
	if collision == nil {
		t.Fatalf("expected non-nil collision vector")
	}
	if collision.ThreatLevel != cli.ThreatGreen {
		t.Errorf("expected GREEN threat level, got: %s", collision.ThreatLevel)
	}
}

func TestDetectCollision_Red(t *testing.T) {
	hullA := &cli.WorktreeHull{
		TaskKey:        "SUB-1191",
		WorktreePath:   "/tmp/worktrees/nomos-SUB-1191",
		TouchedFiles:   []string{"core/types/errors.go"},
		MutatedExports: []string{"NewNomosError"},
		TotalMass:      12.5,
		MeanEntropy:    3.2,
	}
	hullB := &cli.WorktreeHull{
		TaskKey:        "SUB-1192",
		WorktreePath:   "/tmp/worktrees/nomos-SUB-1192",
		TouchedFiles:   []string{"core/types/errors.go"},
		MutatedExports: []string{"NewNomosError"},
		TotalMass:      18.0,
		MeanEntropy:    4.1,
	}

	collision := cli.DetectCollision(hullA, hullB)
	if collision == nil {
		t.Fatalf("expected non-nil collision vector")
	}
	if collision.ThreatLevel != cli.ThreatRed {
		t.Errorf("expected RED threat level due to symbol collision, got: %s", collision.ThreatLevel)
	}
	if len(collision.IntersectedFiles) != 1 || len(collision.IntersectedSymbols) != 1 {
		t.Errorf("expected 1 file and 1 symbol collision, got files=%d symbols=%d",
			len(collision.IntersectedFiles), len(collision.IntersectedSymbols))
	}
}

func TestPhysicsCmd_AndComputeGoFileMass(t *testing.T) {
	dir := t.TempDir()
	sample := filepath.Join(dir, "sample.go")
	src := "package sample\n\n// ExportedAlpha computes deterministic value.\nfunc ExportedAlpha(x int) int {\n\tif x > 0 {\n\t\treturn x * 2\n\t}\n\treturn 0\n}\n"
	if err := os.WriteFile(sample, []byte(src), 0644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	exports := make(map[string]bool)
	imports := make(map[string]bool)
	mass := cli.ComputeGoFileMass(sample, "sample.go", exports, imports)
	if mass.TotalMass <= 0 || !exports["sample.ExportedAlpha"] {
		t.Fatalf("unexpected ComputeGoFileMass result: mass=%+v exports=%v", mass, exports)
	}

	cmd := cli.NewPhysicsCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("nomos physics --json failed: %v", err)
	}
	var rep cli.PhysicsReport
	if err := json.Unmarshal(buf.Bytes(), &rep); err != nil {
		t.Fatalf("unmarshal PhysicsReport JSON: %v", err)
	}
	if rep.MaxThreat == "" {
		t.Fatalf("expected non-empty MaxThreat, got %+v", rep)
	}
}
