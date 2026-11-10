package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func TestHealAccidentalBareConfigAndPolyglotWhitespaceDiscard(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@nomos.local"},
		{"config", "user.name", "Nomos Test"},
	} {
		if out, err := workspace.RunSafeGit(dir, args...); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, string(out))
		}
	}

	pyFile := filepath.Join(dir, "service.py")
	tsFile := filepath.Join(dir, "client.ts")
	if err := os.WriteFile(pyFile, []byte("def add(a, b):\n    return a + b\n"), 0644); err != nil {
		t.Fatalf("write pyFile: %v", err)
	}
	if err := os.WriteFile(tsFile, []byte("export const x = 1;\n"), 0644); err != nil {
		t.Fatalf("write tsFile: %v", err)
	}
	if out, err := workspace.RunSafeGit(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v (%s)", err, string(out))
	}
	if out, err := workspace.RunSafeGit(dir, "commit", "-m", "[SUB-42] feat(app): add polyglot handlers"); err != nil {
		t.Fatalf("git commit: %v (%s)", err, string(out))
	}

	// 1. Accidental core.bare=true on working tree is healed automatically
	if out, err := workspace.RunSafeGit(dir, "config", "--bool", "core.bare", "true"); err != nil {
		t.Fatalf("set core.bare: %v (%s)", err, string(out))
	}
	healAccidentalBareConfig(dir)
	bareOut, bErr := workspace.SafeGitCommand(dir, "config", "--bool", "core.bare").Output()
	if bErr != nil || strings.TrimSpace(string(bareOut)) != "false" {
		t.Fatalf("expected core.bare healed to false, got %q (err=%v)", string(bareOut), bErr)
	}

	// 2. Trailing blank lines on Python + TypeScript files after [SUB-42] commit are discarded without duplicate commit
	if err := os.WriteFile(pyFile, []byte("def add(a, b):\n    return a + b\n\n"), 0644); err != nil {
		t.Fatalf("append blank line py: %v", err)
	}
	if err := os.WriteFile(tsFile, []byte("export const x = 1;   \n\n"), 0644); err != nil {
		t.Fatalf("append blank line ts: %v", err)
	}
	porcelain, _ := workspace.SafeGitCommand(dir, "status", "--porcelain").Output()
	if !discardPureWhitespaceAfterTaskCommit(dir, "SUB-42", string(porcelain)) {
		t.Fatal("expected pure trailing blank-line / EOL-space noise on .py and .ts files to be discarded")
	}
	afterPorcelain, _ := workspace.SafeGitCommand(dir, "status", "--porcelain").Output()
	if strings.TrimSpace(string(afterPorcelain)) != "" {
		t.Fatalf("expected clean working tree after discarding whitespace noise, got: %s", string(afterPorcelain))
	}

	// 3. Substantive polyglot edit is NOT discarded
	if err := os.WriteFile(tsFile, []byte("export const x = 2;\n"), 0644); err != nil {
		t.Fatalf("write substantive ts edit: %v", err)
	}
	substPorcelain, _ := workspace.SafeGitCommand(dir, "status", "--porcelain").Output()
	if discardPureWhitespaceAfterTaskCommit(dir, "SUB-42", string(substPorcelain)) {
		t.Fatal("substantive code edit must never be discarded")
	}
}
