package verify

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func TestPG01PureJSNodeCheckAndTestRunner(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	dir := t.TempDir()
	pkgJSON := `{"name":"pure-js-demo","version":"1.0.0"}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}
	badJS := filepath.Join(dir, "math.js")
	if err := os.WriteFile(badJS, []byte("function add(a, b { return a + b; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, fail := verifyJSAndTypeScriptToolchain(&VerifyContext{RepoRoot: dir, WorktreePath: dir}, dir); fail == nil {
		t.Fatalf("expected pure JS syntax error to fail node --check")
	}

	if err := os.WriteFile(badJS, []byte("module.exports = { add: (a, b) => a + b };\n"), 0644); err != nil {
		t.Fatal(err)
	}
	testJS := "const test = require('node:test');\nconst assert = require('node:assert');\nconst { add } = require('./math.js');\ntest('adds', () => { assert.strictEqual(add(2, 3), 99); });\n"
	if err := os.WriteFile(filepath.Join(dir, "math.test.js"), []byte(testJS), 0644); err != nil {
		t.Fatal(err)
	}
	if _, fail := verifyJSAndTypeScriptToolchain(&VerifyContext{RepoRoot: dir, WorktreePath: dir}, dir); fail == nil {
		t.Fatalf("expected failing node --test suite to fail toolchain gate")
	}

	passTestJS := strings.Replace(testJS, "99", "5", 1)
	if err := os.WriteFile(filepath.Join(dir, "math.test.js"), []byte(passTestJS), 0644); err != nil {
		t.Fatal(err)
	}
	label, fail := verifyJSAndTypeScriptToolchain(&VerifyContext{RepoRoot: dir, WorktreePath: dir}, dir)
	if fail != nil || !strings.Contains(label, "JavaScript(node+test)") {
		t.Fatalf("expected passing pure JS toolchain with JavaScript(node+test), got label=%q fail=%v", label, fail)
	}
}

func TestPG02PythonUnittestFallback(t *testing.T) {
	pyBin, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not installed")
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, dirTests), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "calc.py"), []byte("def double(x):\n    return x * 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	failSuite := "import unittest\nfrom calc import double\nclass CalcTest(unittest.TestCase):\n    def test_double(self):\n        self.assertEqual(double(3), 99)\n"
	if err := os.WriteFile(filepath.Join(dir, dirTests, "test_calc.py"), []byte(failSuite), 0644); err != nil {
		t.Fatal(err)
	}
	if _, fail := runPythonUnittestFallback(context.Background(), pyBin, dir, nil); fail == nil {
		t.Fatalf("expected failing unittest suite to fail")
	}

	passSuite := strings.Replace(failSuite, "99", "6", 1)
	if err := os.WriteFile(filepath.Join(dir, dirTests, "test_calc.py"), []byte(passSuite), 0644); err != nil {
		t.Fatal(err)
	}
	label, fail := runPythonUnittestFallback(context.Background(), pyBin, dir, nil)
	if fail != nil || !strings.Contains(label, "unittest") {
		t.Fatalf("expected passing unittest fallback, got label=%q fail=%v", label, fail)
	}
}

func TestPG03NativeToolchainDiscoveryHelpers(t *testing.T) {
	dir := t.TempDir()
	subProj := filepath.Join(dir, "src", "MyApp")
	if err := os.MkdirAll(subProj, 0755); err != nil {
		t.Fatal(err)
	}
	csProj := filepath.Join(subProj, "MyApp.csproj")
	if err := os.WriteFile(csProj, []byte(`<Project><ItemGroup><PackageReference Include="Microsoft.NET.Test.Sdk" /></ItemGroup></Project>`), 0644); err != nil {
		t.Fatal(err)
	}
	projects, hasSDK := findCSharpProjects(dir)
	if len(projects) != 1 || projects[0] != csProj || !hasSDK {
		t.Fatalf("expected nested .csproj %s with test SDK, got projects=%v hasSDK=%v", csProj, projects, hasSDK)
	}
	rsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rsDir, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rsDir, "src", "lib.rs"), []byte("pub fn ok() -> bool { true }\n#[cfg(test)]\nmod tests { #[test] fn it_works() { assert!(super::ok()); } }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !hasRustTests(rsDir) {
		t.Fatalf("expected inline #[test] in Rust crate to be detected")
	}
}

func TestPG04PolyglotCoverageGateEvaluation(t *testing.T) {
	t.Setenv(envInToolchainGate, "")
	t.Setenv(envSkipToolchainGate, "")
	t.Run("LCOV_And_SimpleCov_Reports", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, dirCoverage), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "app.ts"), []byte("export const x = 1;\n"), 0644); err != nil {
			t.Fatal(err)
		}
		lcov := "SF:app.ts\nLF:10\nLH:9\nend_of_record\n"
		if err := os.WriteFile(filepath.Join(dir, dirCoverage, "lcov.info"), []byte(lcov), 0644); err != nil {
			t.Fatal(err)
		}
		gate := &CoverageGate{}
		res, err := gate.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir, Strict: true})
		if err != nil || !res.Passed || !strings.Contains(res.Message, "LCOV") {
			t.Fatalf("expected LCOV coverage pass, got err=%v res=%+v", err, res)
		}

		rbDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(rbDir, dirCoverage), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rbDir, "item.rb"), []byte("class Item; end\n"), 0644); err != nil {
			t.Fatal(err)
		}
		simpleCov := `{"RSpec":{"coverage":{"/app/item.rb":{"lines":[1,1,0,null,1]}}}}`
		if err := os.WriteFile(filepath.Join(rbDir, dirCoverage, ".resultset.json"), []byte(simpleCov), 0644); err != nil {
			t.Fatal(err)
		}
		rbRes, rbErr := gate.Evaluate(&VerifyContext{RepoRoot: rbDir, WorktreePath: rbDir, Strict: true})
		if rbErr != nil || !rbRes.Passed || !strings.Contains(rbRes.Message, "SimpleCov") {
			t.Fatalf("expected SimpleCov 75%% coverage pass, got err=%v res=%+v", rbErr, rbRes)
		}
	})

	t.Run("PythonStdlibTraceFallback", func(t *testing.T) {
		if _, err := exec.LookPath("python3"); err != nil {
			t.Skip("python3 not installed")
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "ledger.py"), []byte("def add(a, b):\n    return a + b\n"), 0644); err != nil {
			t.Fatal(err)
		}
		testPy := "import unittest\nfrom ledger import add\nclass T(unittest.TestCase):\n    def test_add(self):\n        self.assertEqual(add(1, 2), 3)\n"
		if err := os.WriteFile(filepath.Join(dir, "test_ledger.py"), []byte(testPy), 0644); err != nil {
			t.Fatal(err)
		}
		gate := &CoverageGate{}
		res, err := gate.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir, Strict: true})
		if err != nil || !res.Passed || !strings.Contains(res.Message, "Python(trace)") {
			t.Fatalf("expected stdlib Python trace coverage pass, got err=%v res=%+v", err, res)
		}
	})
}

func TestPG05PolyglotTDDGateStrictAndBrownfield(t *testing.T) {
	strictDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(strictDir, "Cargo.toml"), []byte("[package]\nname = \"demo\"\nversion = \"0.1.0\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(strictDir, "src", "billing")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "invoice.rs"), []byte("pub fn total() -> i32 { 42 }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gate := &TDDGate{}
	res, err := gate.Evaluate(&VerifyContext{RepoRoot: strictDir, WorktreePath: strictDir, Strict: true})
	if err != nil || res.Passed {
		t.Fatalf("expected strict TDDGate to fail on untested Rust module, got %+v", res)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "invoice.rs"), []byte("pub fn total() -> i32 { 42 }\n#[cfg(test)]\nmod tests { #[test] fn t() {} }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	resPass, _ := gate.Evaluate(&VerifyContext{RepoRoot: strictDir, WorktreePath: strictDir, Strict: true})
	if !resPass.Passed {
		t.Fatalf("expected inline Rust #[cfg(test)] to satisfy strict TDDGate, got %+v", resPass)
	}

	bfDir := initTestGitRepo(t)
	if err := os.MkdirAll(filepath.Join(bfDir, dirTests), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bfDir, dirTests, "test_existing.py"), []byte("def test_ok(): assert True\n"), 0644); err != nil {
		t.Fatal(err)
	}
	commitAllTestGit(t, bfDir, "initial baseline")

	if err := os.WriteFile(filepath.Join(bfDir, "new_feature.py"), []byte("def compute():\n    return 7\n"), 0644); err != nil {
		t.Fatal(err)
	}
	bfRes, bfErr := gate.Evaluate(&VerifyContext{RepoRoot: bfDir, WorktreePath: bfDir})
	if bfErr != nil || bfRes.Passed {
		t.Fatalf("expected brownfield TDDGate to catch net-new untested module despite existing tests/ dir, got %+v", bfRes)
	}
}

func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := workspace.SafeGitCommand(dir, "init", "-b", "main").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := workspace.SafeGitCommand(dir, "config", "user.email", "test@nomos.dev").Run(); err != nil {
		t.Fatalf("git email: %v", err)
	}
	if err := workspace.SafeGitCommand(dir, "config", "user.name", "Nomos Test").Run(); err != nil {
		t.Fatalf("git name: %v", err)
	}
	nomosYAML := "project: bf-test\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n"
	if err := os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte(nomosYAML), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func commitAllTestGit(t *testing.T, dir, msg string) {
	t.Helper()
	if err := workspace.SafeGitCommand(dir, "add", ".").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := workspace.SafeGitCommand(dir, "commit", "-m", msg).Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}

func TestSwitchableTDDAndCoverageModesAndLiveProbes(t *testing.T) {
	t.Run("TDDMode_Diff_Vs_Ratchet_Vs_Off", func(t *testing.T) {
		dir := initTestGitRepo(t)
		if err := os.MkdirAll(filepath.Join(dir, dirTests), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, dirTests, "test_other.py"), []byte("def test_x(): assert True\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "legacy.py"), []byte("def calc():\n    return 1\n"), 0644); err != nil {
			t.Fatal(err)
		}
		commitAllTestGit(t, dir, "baseline")

		if err := os.WriteFile(filepath.Join(dir, "legacy.py"), []byte("def calc():\n    return 2\n"), 0644); err != nil {
			t.Fatal(err)
		}
		gate := &TDDGate{}
		// Default brownfield tdd_mode=ratchet passes legacy edit when repo suite exists
		if res, _ := gate.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir}); !res.Passed {
			t.Fatalf("expected ratchet mode to pass legacy edit, got %+v", res)
		}
		// Switch to tdd_mode=diff -> must fail because legacy.py has no paired test and no test was modified!
		diffYAML := "project: bf-test\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n    tdd_mode: diff\n"
		if err := os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte(diffYAML), 0644); err != nil {
			t.Fatal(err)
		}
		if res, _ := gate.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir}); res.Passed {
			t.Fatalf("expected tdd_mode=diff to require paired or modified test on legacy edit")
		}
		// Switch to tdd_mode=off -> must pass
		offYAML := "project: bf-test\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n    tdd_mode: \"off\"\n"
		if err := os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte(offYAML), 0644); err != nil {
			t.Fatal(err)
		}
		if res, _ := gate.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir}); !res.Passed {
			t.Fatalf("expected tdd_mode=off to bypass TDDGate, got %+v", res)
		}
	})

	t.Run("ThresholdIntegrity_RejectsOffInStrictMode", func(t *testing.T) {
		dir := t.TempDir()
		badYAML := "project: strict-demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: true\n    tdd_mode: \"off\"\n"
		if err := os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte(badYAML), 0644); err != nil {
			t.Fatal(err)
		}
		ti := &ThresholdIntegrityGate{}
		if res, _ := ti.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir}); res.Passed {
			t.Fatalf("expected strict_mode=true to reject tdd_mode=off")
		}
	})

	t.Run("NodeLiveV8Coverage_And_CoverageModes", func(t *testing.T) {
		t.Setenv(envInToolchainGate, "")
		t.Setenv(envSkipToolchainGate, "")
		if _, err := exec.LookPath("node"); err != nil {
			t.Skip("node not installed")
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"v8-cov"}`), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "adder.js"), []byte("function add(a, b) {\n  return a + b;\n}\nmodule.exports = { add };\n"), 0644); err != nil {
			t.Fatal(err)
		}
		testJS := "const test = require('node:test');\nconst assert = require('node:assert');\nconst { add } = require('./adder.js');\ntest('add', () => { assert.strictEqual(add(2, 3), 5); });\n"
		if err := os.WriteFile(filepath.Join(dir, "adder.test.js"), []byte(testJS), 0644); err != nil {
			t.Fatal(err)
		}
		cGate := &CoverageGate{}
		res, err := cGate.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir})
		if err != nil || !res.Passed || !strings.Contains(res.Message, "JavaScript/TS(V8)") {
			t.Fatalf("expected live Node V8 coverage pass, got err=%v res=%+v", err, res)
		}
		// Switch to coverage_mode=artifact -> no lcov on disk, skips live probe
		artYAML := "project: v8-cov\ndefault_branch: main\ngovernance:\n  quality_gates:\n    strict_mode: false\n    coverage_mode: artifact\n"
		if err := os.WriteFile(filepath.Join(dir, "nomos.yaml"), []byte(artYAML), 0644); err != nil {
			t.Fatal(err)
		}
		artRes, _ := cGate.Evaluate(&VerifyContext{RepoRoot: dir, WorktreePath: dir})
		if !artRes.Passed || strings.Contains(artRes.Message, "V8") {
			t.Fatalf("expected artifact mode to skip live V8 probe, got %+v", artRes)
		}
	})
}
