package cli

import (
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
)

func TestPlainHandshakeBanner_BrownfieldTopology(t *testing.T) {
	ev := &handshake.HandshakeEvent{
		Seat:       "workstation-local",
		Repository: "commander.js",
		Phase:      types.PhaseEdit,
		ActiveTask: "SUB-1",
		GateCount:  42,
		StrictMode: false,
		BrownfieldTopology: &handshake.BrownfieldTopologyBrief{
			GovernanceMode: "brownfield_ratchet",
			TDDMode:        "ratchet",
			CoverageMode:   "live",
			Languages:      []string{"javascript"},
			TestRunners:    []string{"npm test (node --test)"},
			SourceRoots:    []string{"./", "lib/"},
			TestRoots:      []string{"tests/"},
			SharedModules:  []string{"lib/error.js"},
			Hotspots:       []handshake.BrownfieldHotspot{{Path: "lib/command.js", Lines: 2792}},
		},
	}

	out := formatPlainHandshakeBanner(ev)
	for _, needle := range []string{
		"BROWNFIELD RATCHET (tdd=ratchet, cov=live)",
		"javascript | test: npm test (node --test)",
		"lib/ (tests: tests/)",
		"lib/error.js",
		"lib/command.js (2792 LOC)",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("expected %q in brownfield plain handshake banner, got:\n%s", needle, out)
		}
	}
}
