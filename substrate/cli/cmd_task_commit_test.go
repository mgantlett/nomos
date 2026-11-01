package cli

import (
	"strings"
	"testing"
)

func TestFormatCommitMessage(t *testing.T) {
	opts := &commitCmdOptions{
		summary:     "feat(verify): activate git gates",
		rationale:   "enforce Law 1 and Law 3",
		blastRadius: "verify module",
		mechanics:   "registered gates",
	}

	msg := formatCommitMessage("SUB-1212", opts)

	if !strings.HasPrefix(msg, "[SUB-1212] feat(verify): activate git gates") {
		t.Errorf("unexpected subject line: %s", msg)
	}
	if !strings.Contains(msg, "**Architectural Rationale:**\nenforce Law 1 and Law 3") {
		t.Errorf("missing architectural rationale in: %s", msg)
	}
	if !strings.Contains(msg, "**Blast Radius & Contracts:**\nverify module") {
		t.Errorf("missing blast radius in: %s", msg)
	}
	if !strings.Contains(msg, "**Resolution Mechanics:**\nregistered gates") {
		t.Errorf("missing resolution mechanics in: %s", msg)
	}
}

func TestResolveTaskKeyForCommit_Explicit(t *testing.T) {
	opts := &commitCmdOptions{}
	key, err := resolveTaskKeyForCommit([]string{"sub-1212"}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "SUB-1212" {
		t.Errorf("expected SUB-1212, got %s", key)
	}
}
