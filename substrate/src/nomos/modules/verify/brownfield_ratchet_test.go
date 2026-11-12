package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func buildLegacyGoSource(branches int, extraBody string) string {
	var sb strings.Builder
	sb.WriteString("package engine\n\nfunc LegacyDispatch(code int) int {\n")
	for i := 1; i <= branches; i++ {
		sb.WriteString(fmt.Sprintf("\tif code == %d {\n\t\treturn %d\n\t}\n", i, i*2))
	}
	sb.WriteString("\treturn 0\n}\n\n")
	for i := 0; i < 85; i++ {
		sb.WriteString(fmt.Sprintf("func padUnit%d(x int) int {\n\t// line a\n\t// line b\n\t// line c\n\treturn x + %d\n}\n", i, i))
	}
	sb.WriteString(extraBody)
	return sb.String()
}

func setupBrownfieldGitWorktreeFixture(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	_ = workspace.SafeGitCommand(repo, "init", "-b", "main").Run()
	_ = workspace.SafeGitCommand(repo, "config", "user.email", "test@nomos.local").Run()
	_ = workspace.SafeGitCommand(repo, "config", "user.name", "Nomos Ratchet").Run()

	nomosYAML := "project: legacy-demo\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: false\n    substrate_source_roots:\n      - pkg\n"
	mustWriteGoFixture(t, filepath.Join(repo, "nomos.yaml"), nomosYAML)
	mustWriteGoFixture(t, filepath.Join(repo, "go.mod"), "module example.com/legacydemo\n\ngo 1.22\n")
	mustWriteGoFixture(t, filepath.Join(repo, "pkg", "engine", "legacy.go"), buildLegacyGoSource(22, ""))
	mustWriteGoFixture(t, filepath.Join(repo, "pkg", "engine", "legacy_test.go"),
		"package engine\nimport \"testing\"\nfunc TestLegacy(t *testing.T) {\n\tif LegacyDispatch(1) != 2 { t.Fatal(\"bad\") }\n}\n")

	_ = workspace.SafeGitCommand(repo, "add", ".").Run()
	_ = workspace.SafeGitCommand(repo, "commit", "-m", "chore: baseline").Run()

	wt := filepath.Join(repo, "worktrees", "legacydemo-SUB-1")
	if out, err := workspace.SafeGitCommand(repo, "worktree", "add", "-b", "feature/SUB-1", wt, "main").CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %v (%s)", err, string(out))
	}
	return repo, wt
}

// @verifies TC-PHY-05
func TestBrownfieldMonotonicRatchet_AllowsCleanFixAndBlocksBloat(t *testing.T) {
	repo, wt := setupBrownfieldGitWorktreeFixture(t)
	ctx := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: wt, TaskID: "SUB-1", Phase: "REVIEW"}

	// 1. Empty worktree touches 0 legacy files -> passes immediately
	for _, g := range []vscope.Gate{&MaxFileLOCGate{}, &ComplexityGate{}, &CognitiveDebtGate{}, &DocstringDensityGate{}} {
		res, err := g.Evaluate(ctx)
		if err != nil || !res.Passed {
			t.Fatalf("expected %s to pass on empty brownfield worktree, got res=%+v err=%v", g.Name(), res, err)
		}
	}

	// 2. Clean helper extraction inside 500+ LOC file without bloating LegacyDispatch passes all ratchets
	cleanHelper := "\n// NormalizeInput clamps negative codes to zero (@intent: #VerifyContext).\nfunc NormalizeInput(v int) int {\n\tif v < 0 {\n\t\treturn 0\n\t}\n\treturn v\n}\n"
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "engine", "legacy.go"), buildLegacyGoSource(22, cleanHelper))
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "engine", "legacy_test.go"),
		"package engine\nimport \"testing\"\nfunc TestLegacy(t *testing.T) {\n\tif LegacyDispatch(1) != 2 || NormalizeInput(-3) != 0 { t.Fatal(\"bad\") }\n}\n")

	for _, g := range []vscope.Gate{&MaxFileLOCGate{}, &ComplexityGate{}, &CognitiveDebtGate{}, &DocstringDensityGate{}, &AntiShimGate{}, &CoverageGate{}} {
		ctx.CoverageRep = nil
		res, err := g.Evaluate(ctx)
		if err != nil || !res.Passed {
			t.Fatalf("expected %s to pass on clean modular brownfield fix, got res=%+v err=%v", g.Name(), res, err)
		}
	}

	// 3. Bloating the pre-existing V(G)=23 God function to V(G)=25 fails closed!
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "engine", "legacy.go"), buildLegacyGoSource(24, cleanHelper))
	compRes, err := (&ComplexityGate{}).Evaluate(ctx)
	if err != nil || compRes.Passed || !strings.Contains(compRes.Message, "1-TURN RECIPE") {
		t.Fatalf("expected ComplexityGate to fail closed with 1-TURN RECIPE when legacy God function V(G) increases, got %+v", compRes)
	}
	cdsRes, err := (&CognitiveDebtGate{}).Evaluate(ctx)
	if err != nil || cdsRes.Passed {
		t.Fatalf("expected CognitiveDebtGate to fail closed when file CDS increases above baseline, got %+v", cdsRes)
	}
	assertContainsAllSubstrings(t, cdsRes.Message, "exceeds brownfield ceiling", "dCDS +", "1-TURN RECIPE", "LegacyDispatch (pkg/engine/legacy.go:")

	// 4. Adding a panic("not implemented") shim in modified legacy file fails closed!
	shimHelper := "\n// StubAction is unimplemented.\nfunc StubAction() error {\n\tpanic(\"not implemented\")\n}\n"
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "engine", "legacy.go"), buildLegacyGoSource(22, shimHelper))
	shimRes, err := (&AntiShimGate{}).Evaluate(ctx)
	if err != nil || shimRes.Passed {
		t.Fatalf("expected AntiShimGate to fail closed on newly added panic shim, got %+v", shimRes)
	}

	// 5. Newly created >400-LOC file enforces full statutory ceiling!
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "engine", "legacy.go"), buildLegacyGoSource(22, cleanHelper))
	var hugeNew strings.Builder
	hugeNew.WriteString("package engine\n")
	for i := 0; i < 85; i++ {
		hugeNew.WriteString(fmt.Sprintf("// NewFn%d doc.\nfunc NewFn%d() int {\n\tx := %d\n\treturn x\n}\n", i, i, i))
	}
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "engine", "brand_new_huge.go"), hugeNew.String())
	locRes, err := (&MaxFileLOCGate{}).Evaluate(ctx)
	if err != nil || locRes.Passed {
		t.Fatalf("expected MaxFileLOCGate to reject brand-new >400 LOC file in brownfield mode, got %+v", locRes)
	}
}

func TestBrownfieldPolyglotPythonTDDScoping(t *testing.T) {
	repo := t.TempDir()
	_ = workspace.SafeGitCommand(repo, "init", "-b", "main").Run()
	_ = workspace.SafeGitCommand(repo, "config", "user.email", "test@nomos.local").Run()
	_ = workspace.SafeGitCommand(repo, "config", "user.name", "Nomos Py").Run()

	mustWriteGoFixture(t, filepath.Join(repo, "nomos.yaml"), "project: py-demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n    substrate_source_roots:\n      - src\n")
	mustWriteGoFixture(t, filepath.Join(repo, "pyproject.toml"), "[project]\nname = \"py-demo\"\nversion = \"0.1.0\"\n")
	mustWriteGoFixture(t, filepath.Join(repo, "src", "billing.py"), "def total(x: int) -> int:\n    return x\n")
	_ = workspace.SafeGitCommand(repo, "add", ".").Run()
	_ = workspace.SafeGitCommand(repo, "commit", "-m", "chore: py init").Run()

	wt := filepath.Join(repo, "worktrees", "py-demo-SUB-2")
	if out, err := workspace.SafeGitCommand(repo, "worktree", "add", "-b", "feature/SUB-2", wt, "main").CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v (%s)", err, string(out))
	}
	mustWriteGoFixture(t, filepath.Join(wt, "src", "billing.py"), "def total(x: int) -> int:\n    return max(0, x)\n")

	ctx := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: wt, TaskID: "SUB-2", Phase: "REVIEW"}
	tdd := &TDDGate{}
	failRes, err := tdd.Evaluate(ctx)
	if err != nil || failRes.Passed {
		t.Fatalf("expected TDDGate to fail when modified Python module has zero tests in repo, got %+v", failRes)
	}

	mustWriteGoFixture(t, filepath.Join(wt, "tests", "test_billing.py"), "from src.billing import total\ndef test_total():\n    assert total(-1) == 0\n")
	passRes, err := tdd.Evaluate(ctx)
	if err != nil || !passRes.Passed {
		t.Fatalf("expected TDDGate to pass once Python tests/test_billing.py exists, got %+v err=%v", passRes, err)
	}
}

func TestBrownfieldMagicStringAndForbiddenLiteralRatchet(t *testing.T) {
	repo := t.TempDir()
	_ = workspace.SafeGitCommand(repo, "init", "-b", "main").Run()
	_ = workspace.SafeGitCommand(repo, "config", "user.email", "test@nomos.local").Run()
	_ = workspace.SafeGitCommand(repo, "config", "user.name", "Nomos Py").Run()

	mustWriteGoFixture(t, filepath.Join(repo, "nomos.yaml"), "project: sql-demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n    substrate_source_roots:\n      - sqlglot\n")
	basePy := strings.Repeat("x = \"CUSTOM_MAGIC_TOKEN\"\n", 14) + "fixture = \"/Users/legacy/sample.sql\"\ndef parse(v: str) -> str:\n    return v\n"
	mustWriteGoFixture(t, filepath.Join(repo, "sqlglot", "parser.py"), basePy)
	_ = workspace.SafeGitCommand(repo, "add", ".").Run()
	_ = workspace.SafeGitCommand(repo, "commit", "-m", "chore: baseline").Run()

	wt := filepath.Join(repo, "worktrees", "sql-demo-SUB-3")
	if out, err := workspace.SafeGitCommand(repo, "worktree", "add", "-b", "feature/SUB-3", wt, "main").CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v (%s)", err, string(out))
	}
	mustWriteGoFixture(t, filepath.Join(wt, "sqlglot", "parser.py"), basePy+"\ndef allow_in() -> bool:\n    return True\n")

	ctx := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: wt, TaskID: "SUB-3", Phase: "REVIEW"}
	dupRes, err := (&MagicStringDuplicationGate{}).Evaluate(ctx)
	if err != nil || !dupRes.Passed {
		t.Fatalf("expected MagicStringDuplicationGate to allow pre-existing literal count, got %+v err=%v", dupRes, err)
	}
	litRes, err := (&ForbiddenLiteralsGate{}).Evaluate(ctx)
	if err != nil || !litRes.Passed {
		t.Fatalf("expected ForbiddenLiteralsGate to allow pre-existing fixture literal, got %+v err=%v", litRes, err)
	}

	// Adding a 15th occurrence or a brand-new /Users/ literal must fail closed!
	mustWriteGoFixture(t, filepath.Join(wt, "sqlglot", "parser.py"), basePy+"y = \"CUSTOM_MAGIC_TOKEN\"\nz = \"/Users/newleak/x\"\n")
	dupFail, _ := (&MagicStringDuplicationGate{}).Evaluate(ctx)
	if dupFail.Passed {
		t.Fatalf("expected MagicStringDuplicationGate to block net-new literal duplication")
	}
	litFail, _ := (&ForbiddenLiteralsGate{}).Evaluate(ctx)
	if litFail.Passed {
		t.Fatalf("expected ForbiddenLiteralsGate to block net-new forbidden literal")
	}
}

func assertContainsAllSubstrings(t *testing.T, msg string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected message to contain %q, got:\n%s", want, msg)
		}
	}
}

// @verifies TC-TS-12
func TestPackageFanOutGate_StrictAndBrownfieldRatchet(t *testing.T) {
	repo, wt := setupBrownfieldGitWorktreeFixture(t)
	for i := 0; i < 42; i++ {
		mustWriteGoFixture(t, filepath.Join(repo, "pkg", "crowded", fmt.Sprintf("f%02d.go", i)), "package crowded\n// Fn doc.\nfunc Fn"+fmt.Sprint(i)+"() int { return 1 }\n")
	}
	_ = workspace.SafeGitCommand(repo, "add", ".").Run()
	_ = workspace.SafeGitCommand(repo, "commit", "-m", "chore: crowded dir").Run()
	_ = workspace.SafeGitCommand(wt, "merge", "--ff-only", "main").Run()

	// Touching an existing file in a 42-file brownfield directory passes without growing file count.
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "crowded", "f00.go"), "package crowded\n// Fn0 updated.\nfunc Fn0() int { return 2 }\n")
	ctx := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: wt, TaskID: "SUB-1", Phase: "REVIEW"}
	resPass, err := (&MaxFileLOCGate{}).Evaluate(ctx)
	if err != nil || !resPass.Passed {
		t.Fatalf("expected brownfield 42-file dir to pass without net-new files, got %+v err=%v", resPass, err)
	}

	// Adding a 43rd file in brownfield mode exceeds baseline ceiling 42 and fails closed!
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "crowded", "f42.go"), "package crowded\n// Fn42 doc.\nfunc Fn42() int { return 3 }\n")
	resGrow, err := (&MaxFileLOCGate{}).Evaluate(ctx)
	if err != nil || resGrow.Passed {
		t.Fatalf("expected 43rd file in 42-file brownfield dir to fail closed, got %+v", resGrow)
	}
	assertContainsAllSubstrings(t, resGrow.Message, "pkg/crowded has 43 non-test source files", "exceeds brownfield ceiling 42", "1-TURN RECIPE: extract >=1 cohesive file(s)")

	// Strict mode enforces statutory ceiling 40 even without net-new growth.
	ctxStrict := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: wt, TaskID: "SUB-1", Phase: "REVIEW", Strict: true}
	resStrict, err := (&MaxFileLOCGate{}).Evaluate(ctxStrict)
	if err != nil || resStrict.Passed {
		t.Fatalf("expected strict mode to enforce max 40 package files, got %+v", resStrict)
	}
	assertContainsAllSubstrings(t, resStrict.Message, "pkg/crowded has 43 non-test source files (exceeds max 40)", "1-TURN RECIPE: extract >=3 cohesive file(s)")
}

// @verifies TC-TS-13
func TestRenameAwareRatchet_GitMvPreservesLegacyBaselines(t *testing.T) {
	repo, wt := setupBrownfieldGitWorktreeFixture(t)
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "engine", "sub", "keep_test.go"),
		"package sub\nimport \"testing\"\nfunc TestMoved(t *testing.T) {\n\tif LegacyDispatch(1) != 2 { t.Fatal(\"bad\") }\n}\n")
	if out, err := workspace.SafeGitCommand(wt, "mv", "pkg/engine/legacy.go", "pkg/engine/sub/legacy.go").CombinedOutput(); err != nil {
		t.Fatalf("git mv failed: %v (%s)", err, string(out))
	}
	movedSrc := strings.Replace(buildLegacyGoSource(22, ""), "package engine", "package sub", 1)
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "engine", "sub", "legacy.go"), movedSrc)
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "engine", "legacy_test.go"), "package engine\nimport \"testing\"\nfunc TestRoot(t *testing.T) {}\n")

	ctx := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: wt, TaskID: "SUB-1", Phase: "REVIEW"}
	for _, g := range []vscope.Gate{&MaxFileLOCGate{}, &ComplexityGate{}, &CognitiveDebtGate{}} {
		res, err := g.Evaluate(ctx)
		if err != nil || !res.Passed {
			t.Fatalf("expected %s to preserve brownfield baseline across git mv, got res=%+v err=%v", g.Name(), res, err)
		}
	}

	// Bloating moved function beyond its pre-move baseline still fails closed.
	bloatedMoved := strings.Replace(buildLegacyGoSource(24, ""), "package engine", "package sub", 1)
	mustWriteGoFixture(t, filepath.Join(wt, "pkg", "engine", "sub", "legacy.go"), bloatedMoved)
	compRes, err := (&ComplexityGate{}).Evaluate(ctx)
	if err != nil || compRes.Passed {
		t.Fatalf("expected ComplexityGate to reject post-move complexity bloat above baseline, got %+v", compRes)
	}
}
