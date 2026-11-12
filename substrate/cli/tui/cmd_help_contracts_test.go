package tui

import (
	"strings"
	"testing"
)

func TestHelpUpdateContractsNonEmpty(t *testing.T) {
	if !strings.Contains(HelpUpdateLong, "SHA-256") {
		t.Fatalf("expected helpUpdateLong to mention SHA-256 verification")
	}
	if !strings.Contains(HelpUpdateExample, "nomos update --sync-workspace") {
		t.Fatalf("expected helpUpdateExample to include --sync-workspace")
	}
}
