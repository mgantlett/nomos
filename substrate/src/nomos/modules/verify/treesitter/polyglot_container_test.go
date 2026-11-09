package treesitter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func hermeticPolyglotFixtureFiles() map[string]string {
	return map[string]string{
		"main.go":      "package main\n\n// RunExecution handles the lifecycle loop.\nfunc RunExecution(enabled bool) error {\n\tif enabled {\n\t\treturn nil\n\t}\n\treturn nil\n}\n",
		"service.ts":   "// Service client interface\nexport class ServiceClient {\n\tasync fetchResource(id: string): Promise<string> {\n\t\tif (!id) {\n\t\t\tthrow new Error(\"missing id\");\n\t\t}\n\t\treturn \"ok\";\n\t}\n}\n",
		"helper.js":    "/** Compute metric score */\nfunction computeScore(values) {\n\tlet total = 0;\n\tfor (let i = 0; i < values.length; i++) {\n\t\tif (values[i] > 0) total += values[i];\n\t}\n\treturn total;\n}\n",
		"App.svelte":   "<script lang=\"ts\">\n  let count = 0;\n  function handleClick() {\n    if (count < 100) count += 1;\n  }\n</script>\n\n<main>\n  <h1>Counter</h1>\n  <button on:click={handleClick}>Clicks: {count}</button>\n</main>\n",
		"analyzer.py":  "\"\"\"Analyzer worker module.\"\"\"\n\ndef analyze_payload(payload):\n    if not payload:\n        return None\n    for entry in payload:\n        if entry.get(\"valid\"):\n            return entry\n    return None\n",
		"SPEC.md":      "# Specification Header\n## Core Invariants\n### Gate 1: Tree-sitter AST Verification\nVerification outline content.\n",
		"config.yaml":  "version: \"4.0\"\nruntime:\n  engine: \"wazero\"\n  pure_go: true\n",
		"worker.rs":    "/// Process worker event payload\npub fn process_event(payload: bool) -> bool {\n\tif payload {\n\t\treturn true;\n\t}\n\tfalse\n}\n",
		"native.c":     "/** Calculate checksum value */\nint calculate_checksum(int val) {\n\tif (val > 0) {\n\t\treturn val * 2;\n\t}\n\treturn 0;\n}\n",
		"engine.cpp":   "/** Execute task handler */\nint handle_task(int code) {\n\tif (code == 1) {\n\t\treturn 100;\n\t}\n\treturn 0;\n}\n",
		"index.html":   "<!DOCTYPE html>\n<html>\n  <head><title>Nomos</title></head>\n  <body><h1>Substrate</h1></body>\n</html>\n",
		"style.css":    "body {\n  margin: 0;\n  padding: 0;\n}\n",
		"Service.java": "public class Service {\n\t/** Process request */\n\tpublic int processRequest(int code) {\n\t\tif (code > 0) {\n\t\t\treturn code;\n\t\t}\n\t\treturn 0;\n\t}\n}\n",
		"Pipeline.kt":  "class Pipeline {\n\t/** Run pipeline stage */\n\tfun runStage(active: Boolean): Int {\n\t\tif (active) {\n\t\t\treturn 1\n\t\t}\n\t\treturn 0\n\t}\n}\n",
		"Controller.cs": "public class Controller {\n\t/// Execute controller command\n\tpublic int ExecuteCommand(int input) {\n\t\tif (input > 0) {\n\t\t\treturn input;\n\t\t}\n\t\treturn 0;\n\t}\n}\n",
		"order.rb":      "class Order\n  # Settle invoice balance\n  def settle!(amount)\n    return false unless amount > 0\n    true\n  end\nend\n",
	}
}

func assertPolyglotContainerReport(t *testing.T, ext string, rep *GateEvaluationReport) {
	t.Helper()
	expectedSingleFn := map[string]struct {
		name string
		doc  bool
	}{
		".go":     {"RunExecution", true},
		".js":     {"computeScore", false},
		".svelte": {"handleClick", false},
		".py":     {"analyze_payload", false},
		".rb":     {"settle!", true},
		".rs":     {"process_event", true},
		".c":      {"calculate_checksum", false},
		".cpp":    {"handle_task", false},
	}
	if exp, ok := expectedSingleFn[ext]; ok {
		if len(rep.Functions) != 1 || rep.Functions[0].Name != exp.name {
			t.Errorf("unexpected %s functions: %+v", ext, rep.Functions)
			return
		}
		if exp.doc && !rep.Functions[0].Documented {
			t.Errorf("expected %s function %s to be documented", ext, exp.name)
		}
		return
	}
	if ext == ".ts" && len(rep.Functions) == 0 {
		t.Errorf("expected ts functions extracted")
	}
	if ext == ".md" && (!rep.OutlineVerified || len(rep.HeadingLevels) != 3) {
		t.Errorf("unexpected markdown outline: %+v", rep)
	}
}

// TestPolyglotAST_HermeticWorkspace validates the pure-Go Wazero WebAssembly
// Tree-sitter AST engine against a hermetic multi-language workspace.
func TestPolyglotAST_HermeticWorkspace(t *testing.T) {
	ctx := context.Background()
	engine, err := GetDefaultEngine()
	if err != nil {
		t.Fatalf("failed to acquire default wazero treesitter engine: %v", err)
	}
	if err := engine.ExecuteWasmVerify(ctx); err != nil {
		t.Fatalf("wazero wasm runtime verification failed: %v", err)
	}

	containerDir := t.TempDir()
	files := hermeticPolyglotFixtureFiles()
	for relPath, content := range files {
		fullPath := filepath.Join(containerDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed creating dir for %s: %v", relPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed writing %s: %v", relPath, err)
		}
	}

	for relPath, contentStr := range files {
		t.Run("Language_"+filepath.Ext(relPath), func(t *testing.T) {
			rep, err := engine.ParseAndEvaluate(ctx, relPath, []byte(contentStr))
			if err != nil {
				t.Fatalf("eval failed on %s: %v", relPath, err)
			}
			if !rep.Passed {
				t.Errorf("expected clean container file %s to pass, got failed", relPath)
			}
			assertPolyglotContainerReport(t, filepath.Ext(relPath), rep)
		})
	}
}

func TestPolyglotAST_SyntaxErrorAndAntiShimFailClosed(t *testing.T) {
	ctx := context.Background()
	engine, err := GetDefaultEngine()
	if err != nil {
		t.Fatalf("failed to acquire default wazero treesitter engine: %v", err)
	}

	syntaxErrFiles := map[string]string{
		"bad.go":   "package main\nfunc Incomplete() { ERROR_SYNTAX",
		"bad.ts":   "function Incomplete() { ERROR_SYNTAX",
		"bad.py":   "def broken(\n  ERROR_SYNTAX",
		"bad.rb":   "class Broken\n  def missing_end\n    if ready?\n      run!\nend\n",
		"bad.rs":   "fn broken() { ERROR_SYNTAX",
		"bad.c":    "int broken() { ERROR_SYNTAX",
		"bad.html": "<html><body>ERROR_SYNTAX",
	}
	for name, badCode := range syntaxErrFiles {
		if _, err := engine.ParseAndEvaluate(ctx, name, []byte(badCode)); err == nil {
			t.Errorf("expected syntax error in %s to fail closed, got nil", name)
		}
	}

	shimFiles := map[string]string{
		"shim.go": "package main\nfunc ShimGo() {}",
		"shim.ts": "function ShimTS() {}",
		"shim.py": "def shim_py():\n    pass",
		"shim.rb": "def shim_rb; end",
		"shim.rs": "pub fn shim_rs() {}",
		"shim.c":  "void shim_c() {}",
	}
	for name, shimCode := range shimFiles {
		rep, err := engine.ParseAndEvaluate(ctx, name, []byte(shimCode))
		if err != nil {
			t.Fatalf("unexpected err on %s: %v", name, err)
		}
		if rep.Passed || len(rep.EmptyBlocks) == 0 {
			t.Errorf("expected shim file %s to fail Gate 10 anti-shim, got Passed=%v EmptyBlocks=%v", name, rep.Passed, rep.EmptyBlocks)
		}
	}
}
