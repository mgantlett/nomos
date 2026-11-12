package verify

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

// verifyCompiledGateBehaviorally runs lightweight deterministic checks against
// compiled core gates using isolated temporary fixtures so binary-only proof
// confirms real rejection behavior rather than checking non-nil descriptors.
func verifyCompiledGateBehaviorally() bool {
	tmpDir, err := os.MkdirTemp("", "nomos-inmem-proof-*")
	if err != nil {
		return false
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Construct a stub file triggering AntiShimGate (empty exported body) and
	// MaxFileLOCGate (>400 lines) without shelling out or touching the workspace.
	var sb strings.Builder
	sb.WriteString("package synthetic\n\n")
	sb.WriteString("// Stub triggers AntiShimGate via an empty exported body.\n")
	sb.WriteString("func Stub() {}\n")
	for i := 0; i < 405; i++ {
		sb.WriteString("// padding line to exceed max_file_loc\n")
	}

	probeFile := filepath.Join(tmpDir, "probe.go")
	if err := os.WriteFile(probeFile, []byte(sb.String()), 0644); err != nil {
		return false
	}

	ctx := &vscope.VerifyContext{
		RepoRoot:     tmpDir,
		WorktreePath: tmpDir,
		AllFiles:     true,
		Strict:       false,
	}

	// Both core gates must actively reject the planted violations.
	if !gateRejectsFixture(&AntiShimGate{}, ctx) {
		return false
	}
	if !gateRejectsFixture(&MaxFileLOCGate{}, ctx) {
		return false
	}
	return true
}

// gateRejectsFixture executes a single compiled gate and returns true only when
// the evaluation succeeds without error and reports Passed == false.
func gateRejectsFixture(g vscope.Gate, ctx *vscope.VerifyContext) bool {
	res, err := g.Evaluate(ctx)
	if err != nil || res == nil {
		return false
	}
	return !res.Passed
}

