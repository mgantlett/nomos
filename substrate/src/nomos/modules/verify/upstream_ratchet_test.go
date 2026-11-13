package verify

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/polyrun"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

func initUpstreamGitRepo(t *testing.T, authorEmail string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		if out, err := workspace.RunSafeGit(dir, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
	}
	runGit("init", "-b", "main")
	runGit("config", "user.name", "Test Contributor")
	runGit("config", "user.email", authorEmail)
	for rel, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	runGit("add", ".")
	runGit("commit", "-m", "baseline")
	runGit("checkout", "-b", "feature/SUB-99")
	return dir
}

// @verifies TC-TS-15
func TestUpstreamSurgicalRatchetAllowsInlineEditAndBlocksCollateralMove(t *testing.T) {
	var bigLines []string
	bigLines = append(bigLines, "package sample", "", "func ExistingHelper(x int) int {", "\tif x > 0 {", "\t\treturn x + 1", "\t}", "\treturn x", "}", "")
	for i := 0; i < 420; i++ {
		bigLines = append(bigLines, "// baseline padding line")
	}
	baseGo := strings.Join(bigLines, "\n") + "\n"
	cfgYAML := "project: sample\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: false\n    ratchet_mode: upstream\n"

	repo := initUpstreamGitRepo(t, "contrib@example.org", map[string]string{
		"nomos.yaml": cfgYAML,
		"parser.go":  baseGo,
	})

	// Inline addition of a clean small helper on a >400-LOC baseline file must pass both MaxFileLOCGate and FastFileRadar.
	inlineGo := baseGo + "\n// AddedCleanHelper handles upstream case.\nfunc AddedCleanHelper(v int) int {\n\treturn v * 2\n}\n"
	if err := os.WriteFile(filepath.Join(repo, "parser.go"), []byte(inlineGo), 0644); err != nil {
		t.Fatalf("write inline parser.go: %v", err)
	}
	ctx := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: repo, TaskID: "SUB-99"}
	locRes, err := (&MaxFileLOCGate{}).Evaluate(ctx)
	if err != nil || !locRes.Passed {
		t.Fatalf("expected upstream inline edit to pass MaxFileLOCGate, got res=%+v err=%v", locRes, err)
	}
	radarRes, err := EvaluateFastFileRadar(repo, filepath.Join(repo, "parser.go"))
	if err != nil || len(radarRes.Breaches) > 0 {
		t.Fatalf("expected zero FastFileRadar breaches in ratchet_mode=upstream, got %+v err=%v", radarRes, err)
	}

	// Collateral move of ExistingHelper out of parser.go into a net-new file must fail closed.
	strippedGo := strings.Replace(inlineGo, "func ExistingHelper(x int) int {\n\tif x > 0 {\n\t\treturn x + 1\n\t}\n\treturn x\n}\n", "", 1)
	if err := os.WriteFile(filepath.Join(repo, "parser.go"), []byte(strippedGo), 0644); err != nil {
		t.Fatalf("write stripped parser.go: %v", err)
	}
	newSibling := "package sample\n\nfunc ExistingHelper(x int) int {\n\tif x > 0 {\n\t\treturn x + 1\n\t}\n\treturn x\n}\n"
	if err := os.WriteFile(filepath.Join(repo, "extra.go"), []byte(newSibling), 0644); err != nil {
		t.Fatalf("write extra.go: %v", err)
	}
	ctx2 := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: repo, TaskID: "SUB-99"}
	moveRes, err := (&MaxFileLOCGate{}).Evaluate(ctx2)
	if err != nil || moveRes.Passed || !strings.Contains(moveRes.Message, "collateral extraction of baseline function ExistingHelper") {
		t.Fatalf("expected collateral extraction to fail MaxFileLOCGate, got res=%+v err=%v", moveRes, err)
	}
}

// @verifies TC-TS-16
func TestUpstreamSurgicalRatchetRejectsForeignCopyrightEmail(t *testing.T) {
	cfgYAML := "project: sample\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: false\n    ratchet_mode: upstream\n"
	repo := initUpstreamGitRepo(t, "contrib@example.org", map[string]string{
		"nomos.yaml": cfgYAML,
		"base.go":    "package sample\n\nfunc Base() int { return 1 }\n",
	})

	foreignFile := "// Copyright (c) 2026, Upstream Author <maintainer@upstream.dev>\npackage sample\n\nfunc NetNew() int { return 2 }\n"
	newPath := filepath.Join(repo, "net_new.go")
	if err := os.WriteFile(newPath, []byte(foreignFile), 0644); err != nil {
		t.Fatalf("write net_new.go: %v", err)
	}

	ctx := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: repo, TaskID: "SUB-99"}
	res, err := (&MaxFileLOCGate{}).Evaluate(ctx)
	if err != nil || res.Passed || !strings.Contains(res.Message, "third-party copyright email <maintainer@upstream.dev>") {
		t.Fatalf("expected foreign copyright email on net-new file to fail, got res=%+v err=%v", res, err)
	}
	radar, rErr := EvaluateFastFileRadar(repo, newPath)
	if rErr != nil || len(radar.Breaches) == 0 {
		t.Fatalf("expected FastFileRadar to flag foreign copyright email on net-new file, got %+v err=%v", radar, rErr)
	}
}

// @verifies TC-TS-17
func TestCheckGoSwitchDefaultOrderRejectsNonTerminalDefault(t *testing.T) {
	badSrc := "package p\nfunc f(x int) int {\n\tswitch x {\n\tdefault:\n\t\treturn 0\n\tcase 1:\n\t\treturn 1\n\t}\n}\n"
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "bad.go", badSrc, 0)
	if err != nil {
		t.Fatalf("parse bad.go: %v", err)
	}
	hits := CheckGoSwitchDefaultOrder(node, fset, "bad.go")
	if len(hits) != 1 || !strings.Contains(hits[0], "bad.go:4: non-terminal default clause") {
		t.Fatalf("expected non-terminal default violation at bad.go:4, got %v", hits)
	}

	goodSrc := "package p\nfunc f(x int) int {\n\tswitch x {\n\tcase 1:\n\t\treturn 1\n\tdefault:\n\t\treturn 0\n\t}\n}\n"
	fset2 := token.NewFileSet()
	node2, err := parser.ParseFile(fset2, "good.go", goodSrc, 0)
	if err != nil {
		t.Fatalf("parse good.go: %v", err)
	}
	if hits2 := CheckGoSwitchDefaultOrder(node2, fset2, "good.go"); len(hits2) != 0 {
		t.Fatalf("expected terminal default to pass cleanly, got %v", hits2)
	}
}

// @verifies TC-TS-18
func TestCorpusCommandGateFailsClosedOnRegression(t *testing.T) {
	t.Setenv(polyrun.EnvInToolchainGate, "")
	failCfg := "project: sample\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: false\n    corpus_command: \"echo 'regression in _apt' >&2; exit 1\"\n"
	repo := initUpstreamGitRepo(t, "contrib@example.org", map[string]string{
		"nomos.yaml": failCfg,
	})
	ctx := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: repo, TaskID: "SUB-99"}
	res, err := (&GoToolchainGate{}).Evaluate(ctx)
	if err != nil || res == nil || res.Passed || !strings.Contains(res.Message, "reference corpus regression check failed") {
		t.Fatalf("expected failing corpus_command to fail GoToolchainGate, got res=%+v err=%v", res, err)
	}

	passCfg := "project: sample\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: false\n    corpus_command: \"test -n \\\"$NOMOS_BASELINE_REF\\\"\"\n"
	if err := os.WriteFile(filepath.Join(repo, "nomos.yaml"), []byte(passCfg), 0644); err != nil {
		t.Fatalf("write pass nomos.yaml: %v", err)
	}
	ctxPass := &vscope.VerifyContext{RepoRoot: repo, WorktreePath: repo, TaskID: "SUB-99"}
	resPass, err := (&GoToolchainGate{}).Evaluate(ctxPass)
	if err != nil || resPass == nil || !resPass.Passed || !strings.Contains(resPass.Message, "Corpus(0 regressions)") {
		t.Fatalf("expected passing corpus_command with NOMOS_BASELINE_REF to pass GoToolchainGate, got res=%+v err=%v", resPass, err)
	}
}
