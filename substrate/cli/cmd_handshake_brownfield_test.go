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
			VerifyScope:    "diff",
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
		"BROWNFIELD (scope=diff, tdd=ratchet, cov=live)",
		"javascript | test: npm test (node --test)",
		"lib/ (tests: tests/)",
		"lib/error.js",
		"Core Modules",
		"lib/command.js (2792 LOC)",
	} {
		if !strings.Contains(out, needle) {
			t.Fatalf("expected %q in brownfield plain handshake banner, got:\n%s", needle, out)
		}
	}
	for _, forbidden := range []string{"Ratchet Invariant", "Ratchet Hotspots", "Do not increase function V(G)"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("did not expect upfront warning %q in navigation-first handshake banner, got:\n%s", forbidden, out)
		}
	}
}

func TestPlainHandshakeBanner_TurnZeroTelemetryInjection(t *testing.T) {
	ev := &handshake.HandshakeEvent{
		Seat:             "workstation-local",
		Repository:       "nomos",
		Phase:            types.PhaseEdit,
		ActiveTask:       "SUB-2177",
		GateCount:        42,
		StrictMode:       true,
		ImpactSummary:    []string{"substrate/cli/cmd_hook.go -> paired test: cmd_hook_test.go"},
		LearnedPitfalls:  []string{"[Dead-End in substrate/cli] Rolled back un-merged edit"},
		VerifiedPatterns: []string{"[Pattern in substrate/cli] symbols=[formatPlainHandshakeBanner]"},
		NextActions:      []types.ActionAffordance{{Command: "nomos verify", Summary: "Evaluate 42 gates"}},
	}
	out := formatPlainHandshakeBanner(ev)
	for _, needle := range []string{"AST Impact", "Learned Pitfalls", "[Dead-End in substrate/cli]", "Verified Patterns", "nomos verify"} {
		if !strings.Contains(out, needle) {
			t.Fatalf("expected Turn-0 prefix telemetry %q in banner, got:\n%s", needle, out)
		}
	}
}
