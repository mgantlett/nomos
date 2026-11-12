package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func runTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := workspace.SafeGitCommand(dir, args...)
	cmd.Env = append(cmd.Env,
		"GIT_AUTHOR_NAME=Nomos Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Nomos Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func seedBrownfieldRangeRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	runTestGit(t, dir, "init", "-b", "main")

	legacyPath := filepath.Join(dir, "substrate", "legacy.go")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	var lines []string
	lines = append(lines, "package substrate", "// LegacyHelper is an untouched historical file exceeding max LOC.")
	for i := 0; i < 420; i++ {
		lines = append(lines, fmt.Sprintf("// legacy line %d", i))
	}
	if err := os.WriteFile(legacyPath, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("write legacy.go: %v", err)
	}
	runTestGit(t, dir, "add", ".")
	runTestGit(t, dir, "commit", "-m", "initial brownfield import")
	baseSHA := runTestGit(t, dir, "rev-parse", "HEAD")
	return dir, baseSHA
}

func TestCommitRange_AttestedCommitsSkipBrownfieldFiles(t *testing.T) {
	dir, baseSHA := seedBrownfieldRangeRepo(t)

	cleanFile := filepath.Join(dir, "substrate", "feature.go")
	src := "package substrate\n\n// ComputeSum returns the deterministic sum of two integers.\nfunc ComputeSum(a, b int) int {\n\treturn a + b\n}\n"
	if err := os.WriteFile(cleanFile, []byte(src), 0644); err != nil {
		t.Fatalf("write feature.go: %v", err)
	}
	runTestGit(t, dir, "add", ".")
	runTestGit(t, dir, "commit", "-m", "[SUB-1] feat(substrate): add ComputeSum")
	headSHA := runTestGit(t, dir, "rev-parse", "HEAD")

	receipt := `{"task_key":"SUB-1","phase":"SYNC","dod_gates_passed":42,"sha256_receipt":"abc123def456"}`
	runTestGit(t, dir, "notes", "--ref="+vscope.ReleaseNotesRef, "add", "-m", receipt, headSHA)

	gate := &MaxFileLOCGate{}
	ctx := &vscope.VerifyContext{
		RepoRoot:     dir,
		WorktreePath: dir,
		CommitRange:  baseSHA + ".." + headSHA,
		IsBaseline:   true,
	}
	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected attested commit range to pass without flagging untouched brownfield file, got: %s", res.Message)
	}
}

func TestCommitRange_UnattestedViolationFailsClosed(t *testing.T) {
	dir, baseSHA := seedBrownfieldRangeRepo(t)

	badFile := filepath.Join(dir, "substrate", "unverified.go")
	var badLines []string
	badLines = append(badLines, "package substrate")
	for i := 0; i < 425; i++ {
		badLines = append(badLines, fmt.Sprintf("// unverified line %d", i))
	}
	if err := os.WriteFile(badFile, []byte(strings.Join(badLines, "\n")+"\n"), 0644); err != nil {
		t.Fatalf("write unverified.go: %v", err)
	}
	runTestGit(t, dir, "add", ".")
	runTestGit(t, dir, "commit", "-m", "unverified direct trunk commit")
	headSHA := runTestGit(t, dir, "rev-parse", "HEAD")

	gate := &MaxFileLOCGate{}
	ctx := &vscope.VerifyContext{
		RepoRoot:     dir,
		WorktreePath: dir,
		CommitRange:  baseSHA + ".." + headSHA,
		IsBaseline:   true,
	}
	res, err := gate.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	if res.Passed {
		t.Fatal("expected unattested oversized file in commit range to fail closed")
	}
	if !strings.Contains(res.Message, "unverified.go") || strings.Contains(res.Message, "legacy.go") {
		t.Fatalf("expected failure strictly on unverified.go (not legacy.go), got: %s", res.Message)
	}
}

func TestCollectPolyglotStringLiterals_SvelteScriptScoping(t *testing.T) {
	dir := t.TempDir()
	sveltePath := filepath.Join(dir, "ui", "Widget.svelte")
	if err := os.MkdirAll(filepath.Dir(sveltePath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var markup strings.Builder
	markup.WriteString("<script lang=\"ts\">\n  const repeatedToken = \"duplicate_script_magic\";\n")
	for i := 0; i < 12; i++ {
		markup.WriteString(fmt.Sprintf("  const v%d = \"duplicate_script_magic\";\n", i))
	}
	markup.WriteString("</script>\n")
	for i := 0; i < 15; i++ {
		markup.WriteString("<button type=\"button\"><svg width=\"16\" height=\"16\" viewBox=\"0 0 24 24\"></svg></button>\n")
	}
	if err := os.WriteFile(sveltePath, []byte(markup.String()), 0644); err != nil {
		t.Fatalf("write Widget.svelte: %v", err)
	}

	counts := make(map[string]map[string]int)
	collectPolyglotStringLiterals(sveltePath, counts)

	if counts["ui"]["button"] != 0 || counts["ui"][" height="] != 0 || counts["ui"]["0 0 24 24"] != 0 {
		t.Fatalf("expected HTML/SVG markup attributes outside <script> to be ignored, got counts: %v", counts["ui"])
	}
	if counts["ui"]["duplicate_script_magic"] < 12 {
		t.Fatalf("expected duplicated string literals inside <script> to be counted, got: %d", counts["ui"]["duplicate_script_magic"])
	}
}
