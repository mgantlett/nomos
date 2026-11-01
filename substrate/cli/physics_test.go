package cli_test

import (
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
