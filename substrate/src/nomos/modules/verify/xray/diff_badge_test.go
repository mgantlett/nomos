package xray

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func runXrayGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := workspace.SafeGitCommand(dir, args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func TestScanRepository_DiffModeIgnoresUntouchedLegacyDebt(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "diff-repo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var legacy strings.Builder
	legacy.WriteString("package main\n\nfunc LegacyMonolith(x int) int {\n")
	for i := 0; i < 420; i++ {
		legacy.WriteString("\tx++\n")
	}
	legacy.WriteString("\treturn x\n}\n")
	if err := os.WriteFile(filepath.Join(dir, "legacy.go"), []byte(legacy.String()), 0644); err != nil {
		t.Fatalf("write legacy.go: %v", err)
	}
	runXrayGit(t, dir, "init", "-b", "main")
	runXrayGit(t, dir, "config", "user.email", "dev@example.com")
	runXrayGit(t, dir, "config", "user.name", "Diff Tester")
	runXrayGit(t, dir, "add", "legacy.go")
	runXrayGit(t, dir, "commit", "-m", "chore: legacy baseline")

	runXrayGit(t, dir, "checkout", "-b", "feat/clean-pr")
	if err := os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\n\nfunc CleanAdd(a, b int) int { return a + b }\n"), 0644); err != nil {
		t.Fatalf("write feature.go: %v", err)
	}
	runXrayGit(t, dir, "add", "feature.go")
	runXrayGit(t, dir, "commit", "-m", "feat: add clean helper")
	if err := os.WriteFile(filepath.Join(dir, "draft.go"), []byte("package main\n\nfunc EmptyStub() {}\n"), 0644); err != nil {
		t.Fatalf("write draft.go: %v", err)
	}

	rep, err := ScanRepository(dir, XrayScanOptions{TopHotspots: 10, DiffBaseRef: "main"})
	if err != nil {
		t.Fatalf("diff scan failed: %v", err)
	}
	if rep.DiffBaseRef != "main" || rep.DiffFiles != 2 || rep.FilesScanned != 2 {
		t.Fatalf("expected 2 diff files scanned against main, got %+v", rep)
	}
	if rep.MonolithFiles != 0 {
		t.Fatalf("expected untouched legacy monolith excluded in --diff mode, got monoliths=%d", rep.MonolithFiles)
	}
	if rep.EmptyStubs != 1 {
		t.Fatalf("expected untracked stub detected in --diff mode, got stubs=%d", rep.EmptyStubs)
	}
	if !strings.Contains(FormatTerminal(rep), "Diff Scope") || !strings.Contains(FormatMarkdown(rep), "`main...HEAD`") {
		t.Fatalf("expected Diff Scope rendered in terminal and markdown reports")
	}
	if _, badErr := ScanRepository(dir, XrayScanOptions{DiffBaseRef: "refs/heads/nonexistent-branch"}); badErr == nil {
		t.Fatal("expected unknown --diff base ref to fail closed")
	}
}

func TestFormatBadgeMarkdownAndSVG(t *testing.T) {
	cases := []struct {
		score int
		grade string
		color string
	}{
		{0, "A", "#10b981"},
		{18, "B", "#14b8a6"},
		{40, "C", "#f59e0b"},
		{80, "F", "#ef4444"},
	}
	for _, tc := range cases {
		rep := &XrayReport{ProjectName: "demo", HangoverScore: tc.score, Grade: tc.grade}
		md := FormatBadgeMarkdown(rep)
		if !strings.Contains(md, "https://img.shields.io/badge/Nomos_X--Ray-") || !strings.Contains(md, strings.TrimPrefix(tc.color, "#")) || !strings.Contains(md, "https://sophialabs.ca/xray") {
			t.Fatalf("unexpected markdown badge for %+v: %s", tc, md)
		}
		svg := FormatBadgeSVG(rep)
		if !strings.HasPrefix(svg, "<svg ") || !strings.Contains(svg, tc.color) || !strings.Contains(svg, "Nomos X-Ray") {
			t.Fatalf("unexpected svg badge for %+v: %s", tc, svg)
		}
	}
	if FormatBadgeMarkdown(nil) != "" || FormatBadgeSVG(nil) != "" {
		t.Fatal("expected nil report badge formatters to return empty strings")
	}
}
