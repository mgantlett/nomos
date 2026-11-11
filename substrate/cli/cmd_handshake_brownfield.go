// Package cli implements lean brownfield session handshake formatting.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
)

func renderBrownfieldHandshakeBanner(event *handshake.HandshakeEvent, bf *handshake.BrownfieldTopologyBrief) {
	fmt.Println(renderBrandTitle("🤝 Nomos Brownfield Session Handshake"))
	fmt.Println(uiRule(ruleNarrow))
	fmt.Println(kvLine("Active Repository", 17, uiBoldValueStyle.Render(event.Repository)))
	if event.ActiveTask != "" {
		phaseVal := fmt.Sprintf("%s %s",
			styleSemanticBadge(string(event.Phase)),
			uiMutedStyle.Render("(Task: "+styleTaskKey(event.ActiveTask)+")"))
		fmt.Println(kvLine("Workspace Phase", 17, phaseVal))
	} else {
		phaseVal := fmt.Sprintf("%s %s",
			styleSemanticBadge(string(event.Phase)),
			uiMutedStyle.Render("(Trunk baseline)"))
		fmt.Println(kvLine("Workspace Phase", 17, phaseVal))
	}
	govVal := fmt.Sprintf("BROWNFIELD (scope=%s, tdd=%s, cov=%s, %d gates)",
		effectiveBriefScope(bf), bf.TDDMode, bf.CoverageMode, event.GateCount)
	fmt.Println(kvLine("Governance Mode", 17, uiBoldValueStyle.Render(govVal)))
	for _, item := range buildBrownfieldTopologyPairs(bf) {
		fmt.Println(kvLine(item.label, 17, uiValueStyle.Render(item.value)))
	}
	renderHandshakeMnemeTelemetry(event)
	renderNextActions(os.Stdout, event.NextActions)
	fmt.Println(uiRule(ruleNarrow))
}

func effectiveBriefScope(bf *handshake.BrownfieldTopologyBrief) string {
	if bf != nil && strings.TrimSpace(bf.VerifyScope) != "" {
		return bf.VerifyScope
	}
	return "diff"
}

type brownfieldKV struct {
	label string
	value string
}

func buildBrownfieldTopologyPairs(bf *handshake.BrownfieldTopologyBrief) []brownfieldKV {
	if bf == nil {
		return nil
	}
	var pairs []brownfieldKV
	stack := strings.Join(bf.Languages, ", ")
	if len(bf.TestRunners) > 0 {
		stack += " | test: " + strings.Join(bf.TestRunners, "; ")
	}
	if stack != "" {
		pairs = append(pairs, brownfieldKV{label: "Stack & Runners", value: stack})
	}
	if len(bf.SourceRoots) > 0 {
		topo := strings.Join(bf.SourceRoots, ", ")
		if len(bf.TestRoots) > 0 {
			topo += " (tests: " + strings.Join(bf.TestRoots, ", ") + ")"
		}
		pairs = append(pairs, brownfieldKV{label: "Source Topology", value: topo})
	}
	if len(bf.SharedModules) > 0 {
		pairs = append(pairs, brownfieldKV{label: "Shared Modules", value: strings.Join(bf.SharedModules, ", ")})
	}
	if len(bf.Hotspots) > 0 {
		spots := make([]string, 0, len(bf.Hotspots))
		for _, h := range bf.Hotspots {
			spots = append(spots, fmt.Sprintf("%s (%d LOC)", h.Path, h.Lines))
		}
		pairs = append(pairs, brownfieldKV{label: "Core Modules", value: strings.Join(spots, ", ")})
	}
	return pairs
}

func appendPlainBrownfieldTopology(b *strings.Builder, ev *handshake.HandshakeEvent) {
	if ev == nil || ev.StrictMode || ev.BrownfieldTopology == nil {
		return
	}
	bf := ev.BrownfieldTopology
	b.WriteString(fmt.Sprintf("  Governance Mode   : BROWNFIELD (scope=%s, tdd=%s, cov=%s)\n",
		effectiveBriefScope(bf), bf.TDDMode, bf.CoverageMode))
	for _, item := range buildBrownfieldTopologyPairs(bf) {
		b.WriteString(fmt.Sprintf("  %-17s : %s\n", item.label, item.value))
	}
}

func appendPlainBoundedList(b *strings.Builder, label, icon string, items []string, limit int) {
	if len(items) == 0 {
		return
	}
	b.WriteString(fmt.Sprintf("  %-17s : %d active\n", label, len(items)))
	for i, item := range items {
		if i >= limit {
			break
		}
		b.WriteString(fmt.Sprintf("    %s %s\n", icon, item))
	}
}

// appendPlainTurnZeroTelemetry injects deterministic AST impact, Mneme dead-ends/pitfalls,
// verified blueprints, and HATEOAS next actions into the Turn-0 SessionStart prefix cache.
func appendPlainTurnZeroTelemetry(b *strings.Builder, ev *handshake.HandshakeEvent) {
	if ev == nil {
		return
	}
	appendPlainBoundedList(b, "AST Impact", "↪", ev.ImpactSummary, 4)
	if len(ev.StaleObligations) > 0 {
		paths := make([]string, 0, len(ev.StaleObligations))
		for i, s := range ev.StaleObligations {
			if i >= 3 {
				break
			}
			paths = append(paths, s.SpecPath)
		}
		b.WriteString(fmt.Sprintf("  Stale Specs       : %s\n", strings.Join(paths, ", ")))
	}
	appendPlainBoundedList(b, "Learned Pitfalls", "⚡", ev.LearnedPitfalls, 3)
	appendPlainBoundedList(b, "Verified Patterns", "🧩", ev.VerifiedPatterns, 2)
	if len(ev.NextActions) > 0 {
		b.WriteString("  Next Actions      :\n")
		for _, act := range ev.NextActions {
			b.WriteString(fmt.Sprintf("    → %s (%s)\n", act.Command, act.Summary))
		}
	}
}
