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

func TestScanRepository_ConfigurableThresholdsFromNomosYAMLAndOptions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "custom-bounds")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var src strings.Builder
	src.WriteString("package main\n\nfunc MediumFunc(x int) int {\n")
	for i := 0; i < 420; i++ {
		src.WriteString("\tx++\n")
	}
	src.WriteString("\treturn x\n}\n")
	if err := os.WriteFile(filepath.Join(dir, "medium.go"), []byte(src.String()), 0644); err != nil {
		t.Fatalf("write medium.go: %v", err)
	}

	// Default 400/60 flags both monolith and oversized func.
	defRep, err := ScanRepository(dir, DefaultScanOptions())
	if err != nil {
		t.Fatalf("default scan failed: %v", err)
	}
	if defRep.MonolithFiles != 1 || defRep.OversizedFuncs != 1 || defRep.MaxFileLOCThreshold != 400 {
		t.Fatalf("expected default 400/60 bounds to flag medium.go, got %+v", defRep)
	}

	// nomos.yaml thresholds raise limits to 500/500 cleanly.
	nomosYAML := "project_name: custom-bounds\ngovernance:\n  quality_gates:\n    thresholds:\n      max_file_loc: 500\n      max_func_loc: 500\n      max_cyclomatic_complexity: 20\n"
	if err := os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte(nomosYAML), 0644); err != nil {
		t.Fatalf("write nomos.yaml: %v", err)
	}
	cfgRep, err := ScanRepository(dir, DefaultScanOptions())
	if err != nil {
		t.Fatalf("nomos.yaml scan failed: %v", err)
	}
	if cfgRep.MonolithFiles != 0 || cfgRep.OversizedFuncs != 0 || cfgRep.MaxFileLOCThreshold != 500 || cfgRep.MaxFuncLOCThreshold != 500 || cfgRep.MaxComplexityThreshold != 20 {
		t.Fatalf("expected nomos.yaml 500/500/20 bounds applied, got %+v", cfgRep)
	}
	if !strings.Contains(FormatMarkdown(cfgRep), "`>500` LOC") || !strings.Contains(FormatTerminal(cfgRep), ">500 LOC") {
		t.Fatalf("expected dynamic >500 LOC labels in rendered reports")
	}

	// Explicit option overrides nomos.yaml (tighten back to 300/50).
	ovRep, err := ScanRepository(dir, XrayScanOptions{TopHotspots: 10, MaxFileLOC: 300, MaxFuncLOC: 50, MaxComplexity: 10})
	if err != nil {
		t.Fatalf("override scan failed: %v", err)
	}
	if ovRep.MonolithFiles != 1 || ovRep.OversizedFuncs != 1 || ovRep.MaxFileLOCThreshold != 300 || ovRep.MaxFuncLOCThreshold != 50 || ovRep.MaxComplexityThreshold != 10 {
		t.Fatalf("expected explicit options to override nomos.yaml, got %+v", ovRep)
	}
}
