package verify

import (
	"os"
	"path/filepath"
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
