package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolyglotCoverageAndToolchainProbes(t *testing.T) {
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

	if !isCCppImpactPath("src/engine.cpp") || !isCCppImpactPath("cmakelists.txt") {
		t.Errorf("expected C/C++ impact paths to match")
	}
	if !isJVMImpactPath("src/Main.java") || !isJVMImpactPath("src/App.kt") || !isJVMImpactPath("pom.xml") {
		t.Errorf("expected JVM impact paths to match")
	}
	if !isCSharpImpactPath("src/Program.cs") || !isCSharpImpactPath("app.csproj") {
		t.Errorf("expected C# impact paths to match")
	}
	if !isRubyImpactPath("app/models/order.rb") || !isRubyImpactPath("lib/tasks/deploy.rake") || !isRubyImpactPath("gemfile") {
		t.Errorf("expected Ruby/Rails impact paths to match")
	}

	tmp := t.TempDir()
	if label, fail := verifyCCppToolchain(tmp); label != "" || fail != nil {
		t.Errorf("expected empty C/C++ probe on empty dir")
	}
	if label, fail := verifyJVMToolchain(tmp); label != "" || fail != nil {
		t.Errorf("expected empty JVM probe on empty dir")
	}
	if label, fail := verifyCSharpToolchain(tmp); label != "" || fail != nil {
		t.Errorf("expected empty C# probe on empty dir")
	}
	if label, fail := verifyRubyToolchain(nil, tmp); label != "" || fail != nil {
		t.Errorf("expected empty Ruby probe on empty dir")
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
	_, fail := verifyCCppToolchain(tmp)
	if fail != nil {
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
