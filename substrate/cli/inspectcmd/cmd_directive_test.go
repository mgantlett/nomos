package inspectcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/ncte"
)

func TestDirectiveCmd_LexiconTermAndLint(t *testing.T) {
	lexCmd := NewDirectiveCmd()
	lexBuf := new(bytes.Buffer)
	lexCmd.SetOut(lexBuf)
	lexCmd.SetArgs([]string{"--lexicon", "--domain", "steering", "--json"})
	if err := lexCmd.Execute(); err != nil {
		t.Fatalf("directive --lexicon --domain steering --json failed: %v", err)
	}
	var entries []ncte.VocabularyEntry
	if err := json.Unmarshal(lexBuf.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshal lexicon JSON: %v", err)
	}
	if len(entries) != 6 {
		t.Fatalf("expected 6 steering lexicon entries, got %d", len(entries))
	}

	termCmd := NewDirectiveCmd()
	termBuf := new(bytes.Buffer)
	termCmd.SetOut(termBuf)
	termCmd.SetArgs([]string{"--term", "sandbox"})
	if err := termCmd.Execute(); err != nil {
		t.Fatalf("directive --term sandbox failed: %v", err)
	}
	if !strings.Contains(termBuf.String(), "Worktree") {
		t.Fatalf("expected synonym lookup for sandbox to surface Worktree, got:\n%s", termBuf.String())
	}

	dir := t.TempDir()
	cleanDoc := filepath.Join(dir, "clean.md")
	if err := os.WriteFile(cleanDoc, []byte("# Spec\n\nExecute `nomos verify` inside the isolated worktree.\n"), 0644); err != nil {
		t.Fatalf("write clean.md: %v", err)
	}
	lintCmd := NewDirectiveCmd()
	lintBuf := new(bytes.Buffer)
	lintCmd.SetOut(lintBuf)
	lintCmd.SetArgs([]string{"--lint", cleanDoc, "--json"})
	if err := lintCmd.Execute(); err != nil {
		t.Fatalf("directive --lint clean.md failed: %v", err)
	}
}
