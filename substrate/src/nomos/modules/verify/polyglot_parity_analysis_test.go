package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPG08PolyglotUncheckedErrorsGate(t *testing.T) {
	cases := []struct {
		name      string
		file      string
		code      string
		wantPass  bool
		wantMatch string
	}{
		{"PythonBarePass", "svc.py", "def run():\n    try:\n        work()\n    except Exception:\n        pass\n", false, "except block"},
		{"PythonCommentedPass", "svc.py", "def run():\n    try:\n        work()\n    except Exception:\n        # best-effort cleanup\n        pass\n", true, ""},
		{"TSEmptyCatch", "svc.ts", "export function run() { try { work(); } catch (e) {} }\n", false, "empty catch"},
		{"TSPromiseEmptyCatch", "svc.ts", "export function run() { fetch('/x').catch(() => {}); }\n", false, ".catch()"},
		{"RustBlankLetCall", "svc.rs", "pub fn run() {\n    let _ = persist_record();\n}\n", false, "let _ ="},
		{"RubyRescueNil", "svc.rb", "def run\n  res = risky_call rescue nil\n  res\nend\n", false, "rescue nil"},
	}

	gate := &UncheckedErrorsGate{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.code), 0644); err != nil {
				t.Fatal(err)
			}
			res, err := gate.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir, Strict: true, AllFiles: true})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.Passed != tc.wantPass {
				t.Fatalf("Passed=%v, want %v (msg=%s)", res.Passed, tc.wantPass, res.Message)
			}
			if tc.wantMatch != "" && !strings.Contains(res.Message, tc.wantMatch) {
				t.Fatalf("expected %q in %q", tc.wantMatch, res.Message)
			}
		})
	}
}

func TestPG09PolyglotBoundaryAndDeadCodeGates(t *testing.T) {
	dir := t.TempDir()
	nomosYAML := "project: arch-test\ngovernance:\n  boundary:\n    enforce: true\n    prohibited_packages:\n      - forbidden_sdk\n      - com.secret.internal\n      - legacy/private.h\n  quality_gates:\n    strict_mode: true\n"
	if err := os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte(nomosYAML), 0644); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(dir, "substrate", "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "main.py"), []byte("from forbidden_sdk.client import connect\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "Service.java"), []byte("import com.secret.internal.Vault;\npublic class Service {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "engine.cpp"), []byte("#include \"legacy/private.h\"\nint run() { return 0; }\n"), 0644); err != nil {
		t.Fatal(err)
	}

	bGate := &BoundaryGate{}
	bRes, err := bGate.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir, Strict: true})
	if err != nil || bRes.Passed {
		t.Fatalf("expected BoundaryGate to fail on prohibited polyglot imports, got %+v", bRes)
	}
	for _, want := range []string{"forbidden_sdk.client", "com.secret.internal.Vault", "legacy/private.h"} {
		if !strings.Contains(bRes.Message, want) {
			t.Fatalf("expected %q in BoundaryGate message: %s", want, bRes.Message)
		}
	}

	// DeadCodeGate: orphan_helper.py is never referenced anywhere
	if err := os.WriteFile(filepath.Join(srcDir, "orphan_helper.py"), []byte("def unused():\n    return 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	dGate := &DeadCodeGate{}
	dRes, dErr := dGate.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir, Strict: true})
	if dErr != nil || dRes.Passed || !strings.Contains(dRes.Message, "orphan_helper.py") {
		t.Fatalf("expected DeadCodeGate to flag orphan_helper.py, got err=%v res=%+v", dErr, dRes)
	}
}

func TestPG10PolyglotParameterArityAndSeams(t *testing.T) {
	pyMany := "class Svc:\n    def create(self,\n        a: int, b: int, c: int, d: int, e: int,\n        f: int, g: int, h: int, i: int) -> int:\n        return a\n"
	if v := EvaluateParameterArityViolations("svc.py", []byte(pyMany), 8); len(v) != 1 {
		t.Fatalf("expected 1 Python 9-param arity violation (excluding self), got %v", v)
	}

	rsGenericsOK := "pub fn merge(&mut self, a: HashMap<String, Vec<i32>>, b: Result<(i32, i32), String>, c: i32) -> i32 { c }\n"
	if v := EvaluateParameterArityViolations("lib.rs", []byte(rsGenericsOK), 8); len(v) != 0 {
		t.Fatalf("expected generic commas in Rust not to false-trigger parameter arity, got %v", v)
	}

	rbMany := "def process(p1, p2, p3, p4, p5, p6, p7, p8, p9)\n  p1\nend\n"
	if v := EvaluateParameterArityViolations("job.rb", []byte(rbMany), 8); len(v) != 1 {
		t.Fatalf("expected Ruby 9-param arity violation, got %v", v)
	}

	var longPy strings.Builder
	longPy.WriteString("def orchestrate(items):\n")
	longPy.WriteString("    for item in items:\n")
	for i := 0; i < 14; i++ {
		longPy.WriteString(fmt.Sprintf("        if item == %d:\n            work(%d)\n", i, i))
	}
	longPy.WriteString("    return True\n")
	seams := PrescribeFunctionSeams("orchestrate.py", []byte(longPy.String()), "orchestrate", 1)
	if len(seams) == 0 {
		t.Fatalf("expected Python seam prescriptions for long complex indented function, got none")
	}
}

func TestPG11PolyglotImpactMapAllLanguages(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, body string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Rust with inline #[cfg(test)] + consumer
	mustWrite("src/ledger.rs", "pub fn calc() -> i32 { 1 }\n#[cfg(test)]\nmod tests { #[test] fn ok() {} }\n")
	mustWrite("src/engine.rs", "use crate::ledger;\npub fn run() -> i32 { ledger::calc() }\n")

	// C++ header + consumer + paired test
	mustWrite("include/codec.hpp", "#pragma once\nint encode();\n")
	mustWrite("src/codec_runner.cpp", "#include \"codec.hpp\"\nint go() { return encode(); }\n")
	mustWrite("tests/codec_test.cpp", "int main() { return 0; }\n")

	// C# service + consumer + test project file
	mustWrite("Domain/OrderPricing.cs", "namespace Shop.Domain;\npublic class OrderPricing {}\n")
	mustWrite("Api/Checkout.cs", "using Shop.Domain;\npublic class Checkout { OrderPricing p; }\n")
	mustWrite("Shop.Tests/OrderPricingTests.cs", "public class OrderPricingTests {}\n")

	impacts := ComputeWorkspaceImpact(dir, []string{"src/ledger.rs", "include/codec.hpp", "Domain/OrderPricing.cs"})
	joined := strings.Join(impacts, "\n")
	for _, want := range []string{
		"src/ledger.rs -> paired test: inline #[cfg(test)] | 1-hop dependents: 1 file(s)",
		"include/codec.hpp -> paired test: codec_test.cpp | 1-hop dependents: 1 file(s)",
		"Domain/OrderPricing.cs -> paired test: OrderPricingTests.cs | 1-hop dependents: 1 file(s)",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected ImpactMap line %q in:\n%s", want, joined)
		}
	}
}

func TestPG12BrownfieldFastFileRadar(t *testing.T) {
	dir := initTestGitRepo(t)
	nomosYAML := "project: bf-radar\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n    thresholds:\n      max_file_loc: 15\n      max_func_loc: 15\n      max_cyclomatic_complexity: 10\n      max_cognitive_debt_score: 45\n"
	if err := os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte(nomosYAML), 0644); err != nil {
		t.Fatal(err)
	}

	var legacy strings.Builder
	legacy.WriteString("def legacy_handler(x):\n    path = '/home/' + 'legacy'\n")
	for i := 0; i < 20; i++ {
		legacy.WriteString(fmt.Sprintf("    v_%d = x + %d\n", i, i))
	}
	legacy.WriteString("    return x\n")
	target := filepath.Join(dir, "legacy.py")
	if err := os.WriteFile(target, []byte(legacy.String()), 0644); err != nil {
		t.Fatal(err)
	}
	commitAllTestGit(t, dir, "baseline legacy monolith")

	// Touch file without exceeding baseline LOC (23 lines vs max_file_loc=15)
	sameLOC := strings.Replace(legacy.String(), "v_0 = x + 0", "v_0 = x + 100", 1)
	if err := os.WriteFile(target, []byte(sameLOC), 0644); err != nil {
		t.Fatal(err)
	}
	res, err := EvaluateFastFileRadar(dir, target)
	if err != nil {
		t.Fatalf("EvaluateFastFileRadar error: %v", err)
	}
	if len(res.Breaches) > 0 {
		t.Fatalf("expected zero breaches on non-regressing brownfield baseline edit, got %v", res.Breaches)
	}
	if len(res.Warnings) == 0 {
		t.Fatalf("expected brownfield baseline delta warning on over-limit baseline file")
	}

	// Grow file past baseline LOC -> must trigger Breach
	grown := sameLOC + "    extra_1 = 1\n    extra_2 = 2\n"
	if err := os.WriteFile(target, []byte(grown), 0644); err != nil {
		t.Fatal(err)
	}
	resBreach, _ := EvaluateFastFileRadar(dir, target)
	if len(resBreach.Breaches) == 0 {
		t.Fatalf("expected breach when exceeding brownfield baseline LOC, got warnings=%v", resBreach.Warnings)
	}
}

