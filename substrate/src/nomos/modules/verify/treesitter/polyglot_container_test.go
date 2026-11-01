package treesitter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestPolyglotAST_HermeticWorkspace validates the pure-Go Wazero WebAssembly
// Tree-sitter AST engine against a hermetic multi-language workspace containing
// Go, TypeScript, JavaScript, Python, Svelte, Markdown, and YAML sources.
func TestPolyglotAST_HermeticWorkspace(t *testing.T) {
	ctx := context.Background()
	engine, err := GetDefaultEngine()
	if err != nil {
		t.Fatalf("failed to acquire default wazero treesitter engine: %v", err)
	}

	// 1. Verify hermetic Wazero WebAssembly runtime execution
	if err := engine.ExecuteWasmVerify(ctx); err != nil {
		t.Fatalf("wazero wasm runtime verification failed: %v", err)
	}

	// 2. Scaffold clean-room polyglot inception testbed container
	containerDir := t.TempDir()

	files := map[string]string{
		"main.go": `package main

// RunExecution handles the lifecycle loop.
func RunExecution(enabled bool) error {
	if enabled {
		return nil
	}
	return nil
}
`,
		"service.ts": `// Service client interface
export class ServiceClient {
	async fetchResource(id: string): Promise<string> {
		if (!id) {
			throw new Error("missing id");
		}
		return "ok";
	}
}
`,
		"helper.js": `/** Compute metric score */
function computeScore(values) {
	let total = 0;
	for (let i = 0; i < values.length; i++) {
		if (values[i] > 0) total += values[i];
	}
	return total;
}
`,
		"App.svelte": `<script lang="ts">
  let count = 0;
  function handleClick() {
    if (count < 100) count += 1;
  }
</script>

<main>
  <h1>Counter</h1>
  <button on:click={handleClick}>Clicks: {count}</button>
</main>
`,
		"analyzer.py": `"""Analyzer worker module."""

def analyze_payload(payload):
    if not payload:
        return None
    for entry in payload:
        if entry.get("valid"):
            return entry
    return None
`,
		"SPEC.md": `# Specification Header
## Core Invariants
### Gate 1: Tree-sitter AST Verification
Verification outline content.
`,
		"config.yaml": `version: "4.0"
runtime:
  engine: "wazero"
  pure_go: true
languages:
  - go
  - typescript
  - javascript
  - svelte
  - python
  - rust
  - c
  - cpp
  - html
  - css
  - markdown
  - yaml
`,
		"worker.rs": `/// Process worker event payload
pub fn process_event(payload: bool) -> bool {
	if payload {
		return true;
	}
	false
}
`,
		"native.c": `/** Calculate checksum value */
int calculate_checksum(int val) {
	if (val > 0) {
		return val * 2;
	}
	return 0;
}
`,
		"engine.cpp": `/** Execute task handler */
int handle_task(int code) {
	if (code == 1) {
		return 100;
	}
	return 0;
}
`,
		"index.html": `<!DOCTYPE html>
<html>
  <head><title>Nomos</title></head>
  <body><h1>Substrate</h1></body>
</html>
`,
		"style.css": `body {
  margin: 0;
  padding: 0;
}
`,
	}

	for relPath, content := range files {
		fullPath := filepath.Join(containerDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed creating dir for %s: %v", relPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed writing %s: %v", relPath, err)
		}
	}

	// 3. Test evaluation across every language in the container
	for relPath, contentStr := range files {
		t.Run("Language_"+filepath.Ext(relPath), func(t *testing.T) {
			rep, err := engine.ParseAndEvaluate(ctx, relPath, []byte(contentStr))
			if err != nil {
				t.Fatalf("eval failed on %s: %v", relPath, err)
			}
			if !rep.Passed {
				t.Errorf("expected clean container file %s to pass, got failed", relPath)
			}

			switch filepath.Ext(relPath) {
			case ".go":
				if len(rep.Functions) != 1 || rep.Functions[0].Name != "RunExecution" {
					t.Errorf("unexpected go functions: %+v", rep.Functions)
				}
				if !rep.Functions[0].Documented {
					t.Errorf("expected RunExecution to be documented")
				}
			case ".ts":
				if len(rep.Functions) == 0 {
					t.Errorf("expected ts functions extracted")
				}
			case ".js":
				if len(rep.Functions) != 1 || rep.Functions[0].Name != "computeScore" {
					t.Errorf("unexpected js functions: %+v", rep.Functions)
				}
			case ".svelte":
				if len(rep.Functions) != 1 || rep.Functions[0].Name != "handleClick" {
					t.Errorf("unexpected svelte functions: %+v", rep.Functions)
				}
			case ".py":
				if len(rep.Functions) != 1 || rep.Functions[0].Name != "analyze_payload" {
					t.Errorf("unexpected python functions: %+v", rep.Functions)
				}
			case ".rs":
				if len(rep.Functions) != 1 || rep.Functions[0].Name != "process_event" {
					t.Errorf("unexpected rust functions: %+v", rep.Functions)
				}
				if !rep.Functions[0].Documented {
					t.Errorf("expected process_event to be documented")
				}
			case ".c":
				if len(rep.Functions) != 1 || rep.Functions[0].Name != "calculate_checksum" {
					t.Errorf("unexpected c functions: %+v", rep.Functions)
				}
			case ".cpp":
				if len(rep.Functions) != 1 || rep.Functions[0].Name != "handle_task" {
					t.Errorf("unexpected cpp functions: %+v", rep.Functions)
				}
			case ".md":
				if !rep.OutlineVerified || len(rep.HeadingLevels) != 3 {
					t.Errorf("unexpected markdown outline: %+v", rep)
				}
			}
		})
	}

	// 4. Test failure cases inside container: syntax errors fail closed
	t.Run("SyntaxError_FailClosed", func(t *testing.T) {
		syntaxErrFiles := map[string]string{
			"bad.go":   "package main\nfunc Incomplete() { ERROR_SYNTAX",
			"bad.ts":   "function Incomplete() { ERROR_SYNTAX",
			"bad.py":   "def broken(\n  ERROR_SYNTAX",
			"bad.rs":   "fn broken() { ERROR_SYNTAX",
			"bad.c":    "int broken() { ERROR_SYNTAX",
			"bad.html": "<html><body>ERROR_SYNTAX",
		}
		for name, badCode := range syntaxErrFiles {
			_, err := engine.ParseAndEvaluate(ctx, name, []byte(badCode))
			if err == nil {
				t.Errorf("expected syntax error in %s to fail closed, got nil", name)
			}
		}
	})

	// 5. Test anti-shim detection across container files
	t.Run("AntiShim_Detection", func(t *testing.T) {
		shimFiles := map[string]string{
			"shim.go": "package main\nfunc ShimGo() {}",
			"shim.ts": "function ShimTS() {}",
			"shim.py": "def shim_py():\n    pass",
			"shim.rs": "pub fn shim_rs() {}",
			"shim.c":  "void shim_c() {}",
		}
		for name, shimCode := range shimFiles {
			rep, err := engine.ParseAndEvaluate(ctx, name, []byte(shimCode))
			if err != nil {
				t.Fatalf("unexpected err on %s: %v", name, err)
			}
			if rep.Passed {
				t.Errorf("expected shim file %s to fail Gate 10 anti-shim", name)
			}
			if len(rep.EmptyBlocks) == 0 {
				t.Errorf("expected empty block identified in %s", name)
			}
		}
	})
}
