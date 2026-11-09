package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLCOVAndCoberturaCoverage(t *testing.T) {
	lcovSample := []byte("TN:\nSF:src/index.ts\nLF:20\nLH:17\nend_of_record\nSF:src/util.ts\nLF:30\nLH:23\nend_of_record\n")
	pct, ok := ParseLCOVCoverage(lcovSample)
	if !ok || pct != 80.0 {
		t.Fatalf("expected 80.0%% LCOV coverage, got pct=%v ok=%v", pct, ok)
	}
	if _, okEmpty := ParseLCOVCoverage([]byte("TN:\n")); okEmpty {
		t.Errorf("expected empty LCOV to return ok=false")
	}

	coberturaSample := []byte(`<?xml version="1.0"?><coverage line-rate="0.825" branch-rate="0.75"></coverage>`)
	cPct, cOk := ParseCoberturaCoverage(coberturaSample)
	if !cOk || cPct != 82.5 {
		t.Fatalf("expected 82.5%% Cobertura coverage, got pct=%v ok=%v", cPct, cOk)
	}
	if _, cBad := ParseCoberturaCoverage([]byte(`<coverage/>`)); cBad {
		t.Errorf("expected missing line-rate to return ok=false")
	}
}

func TestPolyglotImpactPaths(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string) bool
		hits []string
	}{
		{"C/C++", isCCppImpactPath, []string{"src/engine.cpp", "cmakelists.txt"}},
		{"JVM", isJVMImpactPath, []string{"src/Main.java", "src/App.kt", "pom.xml"}},
		{"C#", isCSharpImpactPath, []string{"src/Program.cs", "app.csproj"}},
		{"Ruby/Rails", isRubyImpactPath, []string{"app/models/order.rb", "lib/tasks/deploy.rake", "gemfile"}},
	}
	for _, tc := range cases {
		for _, p := range tc.hits {
			if !tc.fn(p) {
				t.Errorf("expected %s impact path %q to match", tc.name, p)
			}
		}
	}
}

func TestPolyglotToolchainProbesOnTempWorkspace(t *testing.T) {
	tmp := t.TempDir()
	probes := []struct {
		name string
		run  func(string) (string, *GateResult)
	}{
		{"C/C++", verifyCCppToolchain},
		{"JVM", verifyJVMToolchain},
		{"C#", verifyCSharpToolchain},
		{"Ruby", func(dir string) (string, *GateResult) { return verifyRubyToolchain(nil, dir) }},
	}
	for _, p := range probes {
		if label, fail := p.run(tmp); label != "" || fail != nil {
			t.Errorf("expected empty %s probe on empty dir, got label=%q fail=%+v", p.name, label, fail)
		}
	}

	railsTest := filepath.Join(tmp, "test", "models")
	if err := os.MkdirAll(railsTest, 0755); err != nil {
		t.Fatalf("mkdir rails test dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(railsTest, "order_test.rb"), []byte("class OrderTest; end\n"), 0644); err != nil {
		t.Fatalf("write order_test.rb: %v", err)
	}
	if paired := resolvePairedTestStatus(tmp, "app/models/order.rb"); paired != "paired test: order_test.rb" {
		t.Errorf("expected Rails Minitest paired test resolution, got %q", paired)
	}

	cmakeFile := filepath.Join(tmp, "CMakeLists.txt")
	if err := os.WriteFile(cmakeFile, []byte("cmake_minimum_required(VERSION 3.10)\n"), 0644); err != nil {
		t.Fatalf("write CMakeLists.txt: %v", err)
	}
	if _, fail := verifyCCppToolchain(tmp); fail != nil {
		t.Errorf("unexpected C/C++ cmake probe failure: %+v", fail)
	}
}

func writeFakeExec(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("#!/usr/bin/env sh\n"+body+"\n"), 0755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func seedFakeRailsRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	binPath := t.TempDir()
	writeFakeExec(t, filepath.Join(binPath, "ruby"), "exit 0")

	if err := os.MkdirAll(filepath.Join(repo, "app", "models"), 0755); err != nil {
		t.Fatalf("mkdir app/models: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "app", "models", "order.rb"), []byte("class Order\n  def valid?\n    true\n  end\nend\n"), 0644); err != nil {
		t.Fatalf("write order.rb: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "test", "models"), 0755); err != nil {
		t.Fatalf("mkdir test/models: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "test", "models", "order_test.rb"), []byte("class OrderTest\n  def test_valid\n    true\n  end\nend\n"), 0644); err != nil {
		t.Fatalf("write order_test.rb: %v", err)
	}
	return repo, binPath
}

func TestVerifyRubyToolchain_Rails8MinitestAndRakeAutoRunner(t *testing.T) {
	repo, binPath := seedFakeRailsRepo(t)
	t.Setenv("PATH", binPath+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Case 1: Rails 8 bin/rails test passes with 0 skips -> Ruby(compile+minitest)
	writeFakeExec(t, filepath.Join(repo, "bin", "rails"),
		`if [ "$1" != "test" ]; then exit 2; fi; echo "8 runs, 14 assertions, 0 failures, 0 errors, 0 skips"; exit 0`)
	label, fail := verifyRubyToolchain(nil, repo)
	if fail != nil || label != "Ruby(compile+minitest)" {
		t.Fatalf("expected Ruby(compile+minitest) pass, got label=%q fail=%+v", label, fail)
	}

	// Case 2: Skipped Minitest (exit 0 but "1 skips") fails closed
	writeFakeExec(t, filepath.Join(repo, "bin", "rails"),
		`echo "8 runs, 13 assertions, 0 failures, 0 errors, 1 skips"; exit 0`)
	_, skipFail := verifyRubyToolchain(nil, repo)
	if skipFail == nil || !strings.Contains(skipFail.Message, "skipped tests") {
		t.Fatalf("expected skipped Minitest output to fail closed, got %+v", skipFail)
	}

	// Case 3: Failing Minitest assertion (exit 1) fails closed
	writeFakeExec(t, filepath.Join(repo, "bin", "rails"),
		`echo "Failure:\nOrderTest#test_valid [test/models/order_test.rb:6]: Expected false to be truthy."; exit 1`)
	_, assertFail := verifyRubyToolchain(nil, repo)
	if assertFail == nil || !strings.Contains(assertFail.Message, "Minitest suite failed") {
		t.Fatalf("expected failing Minitest assertion to fail closed, got %+v", assertFail)
	}

	// Case 4: Gemfile + Rakefile without bin/rails executes bundle exec rake test
	if err := os.RemoveAll(filepath.Join(repo, "bin")); err != nil {
		t.Fatalf("remove bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Gemfile"), []byte("source 'https://rubygems.org'\n"), 0644); err != nil {
		t.Fatalf("write Gemfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "Rakefile"), []byte("task :test\n"), 0644); err != nil {
		t.Fatalf("write Rakefile: %v", err)
	}
	writeFakeExec(t, filepath.Join(binPath, "bundle"),
		`if [ "$1 $2 $3" != "exec rake test" ]; then exit 3; fi; echo "4 runs, 8 assertions, 0 failures, 0 errors, 0 skips"; exit 0`)
	rakeLabel, rakeFail := verifyRubyToolchain(nil, repo)
	if rakeFail != nil || rakeLabel != "Ruby(compile+minitest)" {
		t.Fatalf("expected bundle exec rake test -> Ruby(compile+minitest), got label=%q fail=%+v", rakeLabel, rakeFail)
	}
}
