package cli

import (
	"strings"
	"testing"
)

func TestHelpUpdateContractsNonEmpty(t *testing.T) {
	if !strings.Contains(helpUpdateLong, "SHA-256") {
		t.Fatalf("expected helpUpdateLong to mention SHA-256 verification")
	}
	if !strings.Contains(helpUpdateExample, "nomos update --sync-workspace") {
		t.Fatalf("expected helpUpdateExample to include --sync-workspace")
	}
}

