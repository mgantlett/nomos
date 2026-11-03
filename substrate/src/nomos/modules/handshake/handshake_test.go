package handshake_test

import (
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/handshake"
)

func TestBuildAndValidateHandshakeEvent(t *testing.T) {
	event := &handshake.HandshakeEvent{
		Seat:             "dev-seat-1",
		Repository:       "nomos",
		Plane:            "substrate",
		Phase:            types.PhaseEdit,
		ActiveTask:       "SUB-1197",
		StepBudget:       100,
		TokensUsed:       12,
		DatabasesHealthy: true,
		GateCount:        42,
		ReadyWavefront:   []string{"SUB-1198", "SUB-1199"},
	}

	if err := handshake.ValidateHandshakeEvent(event); err != nil {
		t.Fatalf("expected valid handshake event, got error: %v", err)
	}
}

func TestValidateHandshakeEvent_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		event *handshake.HandshakeEvent
	}{
		{"nil event", nil},
		{"empty seat", &handshake.HandshakeEvent{Seat: "", Repository: "nomos", Phase: types.PhaseIdle}},
		{"empty repo", &handshake.HandshakeEvent{Seat: "seat-1", Repository: "", Phase: types.PhaseIdle}},
		{"invalid phase", &handshake.HandshakeEvent{Seat: "seat-1", Repository: "nomos", Phase: types.Phase("INVALID")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handshake.ValidateHandshakeEvent(tt.event); err == nil {
				t.Errorf("expected validation failure for %s, but got nil", tt.name)
			}
		})
	}
}

func TestEmbeddedDirectives_ListAndGet(t *testing.T) {
	names, err := handshake.ListEmbeddedDirectives()
	if err != nil {
		t.Fatalf("unexpected error listing embedded directives: %v", err)
	}
	if len(names) < 16 {
		t.Errorf("expected at least 16 embedded Markdown directives, got %d: %v", len(names), names)
	}
	for _, n := range names {
		if !strings.HasSuffix(n, ".md") {
			t.Errorf("expected only .md directives in embedded list, got %q", n)
		}
	}

	// Test exact match
	content, err := handshake.GetEmbeddedDirective("edit_01_deprecated_terms.md")
	if err != nil {
		t.Fatalf("failed retrieving exact directive: %v", err)
	}
	if !strings.Contains(string(content), "Deprecated Terms") {
		t.Errorf("expected directive content to contain 'Deprecated Terms', got: %s", string(content[:100]))
	}

	// Test prefix / extensionless match
	contentPrefix, err := handshake.GetEmbeddedDirective("edit_01")
	if err != nil {
		t.Fatalf("failed retrieving directive by prefix 'edit_01': %v", err)
	}
	if string(contentPrefix) != string(content) {
		t.Errorf("expected prefix match to return same content as exact match")
	}

	// Test non-existent directive
	_, err = handshake.GetEmbeddedDirective("nonexistent_directive_xyz")
	if err == nil {
		t.Errorf("expected error for non-existent directive, got nil")
	}
}

func TestLoadPhaseDirectives_EmbeddedFallback(t *testing.T) {
	// With empty/non-existent repoRoot, it should fall back to embedded directives
	editDirectives := handshake.LoadPhaseDirectives("", types.PhaseEdit)
	if len(editDirectives) != 6 {
		t.Errorf("expected 6 edit directives from embedded fallback, got %d: %v", len(editDirectives), editDirectives)
	}

	planDirectives := handshake.LoadPhaseDirectives("", types.PhasePlan)
	if len(planDirectives) != 3 {
		t.Errorf("expected 3 plan directives from embedded fallback, got %d: %v", len(planDirectives), planDirectives)
	}

	reviewDirectives := handshake.LoadPhaseDirectives("", types.PhaseReview)
	if len(reviewDirectives) != 2 {
		t.Errorf("expected 2 review directives from embedded fallback, got %d: %v", len(reviewDirectives), reviewDirectives)
	}

	syncDirectives := handshake.LoadPhaseDirectives("", types.PhaseSync)
	if len(syncDirectives) != 1 {
		t.Errorf("expected 1 sync directive from embedded fallback, got %d: %v", len(syncDirectives), syncDirectives)
	}

	for _, phase := range types.AllPhases {
		dirs := handshake.LoadPhaseDirectives("", phase)
		if len(dirs) == 0 {
			t.Fatalf("expected non-empty directives for phase %s", phase)
		}
		for _, d := range dirs {
			if _, err := handshake.GetEmbeddedDirective(d); err != nil {
				t.Errorf("phase %s returned unresolvable directive %q: %v", phase, d, err)
			}
		}
	}
}

func TestListDirectives_Metadata(t *testing.T) {
	list, err := handshake.ListDirectives("")
	if err != nil {
		t.Fatalf("unexpected error listing directives: %v", err)
	}
	if len(list) < 16 {
		t.Errorf("expected at least 16 directives in list, got %d", len(list))
	}
	for _, d := range list {
		if !strings.HasSuffix(d.Name, ".md") || strings.TrimSpace(d.Summary) == "" {
			t.Errorf("expected .md directive with non-empty summary, got %+v", d)
		}
	}

	foundTaxonomy := false
	for _, d := range list {
		if d.Name == "00_index_and_taxonomy.md" {
			foundTaxonomy = true
			if d.Title == "" {
				t.Errorf("expected non-empty title for 00_index_and_taxonomy.md")
			}
			if d.Source != "embedded" {
				t.Errorf("expected source to be 'embedded', got %q", d.Source)
			}
		}
	}
	if !foundTaxonomy {
		t.Errorf("00_index_and_taxonomy.md not found in ListDirectives")
	}
}
