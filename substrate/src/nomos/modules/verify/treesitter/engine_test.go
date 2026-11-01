package treesitter

import (
	"context"
	"strings"
	"testing"
)

func TestTreeSitterEngine_Vectors(t *testing.T) {
	ctx := context.Background()
	engine, err := NewTreeSitterEngine(ctx)
	if err != nil {
		t.Fatalf("failed to initialize treesitter engine: %v", err)
	}
	defer engine.Close(ctx)

	t.Run("VEC-TS-01 Go Valid Doc and Branch", func(t *testing.T) {
		src := []byte("// Exec runs task\nfunc Exec() error {\n\tif err := step(); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}")
		rep, err := engine.ParseAndEvaluate(ctx, "exec.go", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rep.Passed {
			t.Errorf("expected report to pass")
		}
		if len(rep.Functions) != 1 {
			t.Fatalf("expected 1 function, got %d", len(rep.Functions))
		}
		fn := rep.Functions[0]
		if fn.Name != "Exec" || !fn.Documented || fn.Complexity != 2 {
			t.Errorf("unexpected metrics: %+v", fn)
		}
	})

	t.Run("VEC-TS-02 Go Syntax Error Fail Closed", func(t *testing.T) {
		src := []byte("func Incomplete() { ERROR_SYNTAX")
		_, err := engine.ParseAndEvaluate(ctx, "err.go", src)
		if err == nil {
			t.Errorf("expected fail-closed error on syntax error, got nil")
		}
	})

	t.Run("VEC-TS-03 TypeScript Arrow Function", func(t *testing.T) {
		src := []byte("export const handle = (req: Req) => {\n\tif (a && b || c) {\n\t\treturn d;\n\t}\n};")
		rep, err := engine.ParseAndEvaluate(ctx, "handler.ts", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rep.Functions) != 1 {
			t.Fatalf("expected 1 function, got %d", len(rep.Functions))
		}
		fn := rep.Functions[0]
		if fn.Name != "handle" || fn.Complexity < 4 {
			t.Errorf("expected handle with complexity >= 4, got: %+v", fn)
		}
	})

	t.Run("VEC-TS-04 Python Branches", func(t *testing.T) {
		src := []byte("def dispatch(event):\n    if event:\n        for item in event:\n            pass")
		rep, err := engine.ParseAndEvaluate(ctx, "event.py", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rep.Functions) != 1 {
			t.Fatalf("expected 1 function, got %d", len(rep.Functions))
		}
		fn := rep.Functions[0]
		if fn.Name != "dispatch" || fn.Complexity != 3 {
			t.Errorf("expected dispatch with complexity 3, got: %+v", fn)
		}
	})

	t.Run("VEC-TS-05 Go Complexity Exceeded", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("package test\nfunc Complex(x int) {\n")
		for i := 0; i < 16; i++ {
			b.WriteString("\tif x == 1 { return }\n")
		}
		b.WriteString("}\n")
		rep, err := engine.ParseAndEvaluate(ctx, "complex.go", []byte(b.String()))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rep.Passed {
			t.Errorf("expected report to fail due to cyclomatic complexity > 15")
		}
		if rep.Functions[0].Complexity != 17 {
			t.Errorf("expected complexity 17, got %d", rep.Functions[0].Complexity)
		}
	})

	t.Run("VEC-TS-06 TypeScript JSDoc Adjacency", func(t *testing.T) {
		src := []byte("/** Valid JSDoc */\nfunction calc(): void {\n\treturn;\n}")
		rep, err := engine.ParseAndEvaluate(ctx, "calc.ts", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rep.Functions) != 1 || !rep.Functions[0].Documented {
			t.Errorf("expected documented function, got: %+v", rep.Functions)
		}
	})

	t.Run("VEC-TS-07 Go Empty Block Shim Violation", func(t *testing.T) {
		src := []byte("package test\nfunc Shim() {}")
		rep, err := engine.ParseAndEvaluate(ctx, "shim.go", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rep.Passed {
			t.Errorf("expected report to fail due to Gate 10 anti-shim violation")
		}
		if len(rep.EmptyBlocks) != 1 || rep.EmptyBlocks[0] != "Shim" {
			t.Errorf("expected empty block Shim detected, got: %+v", rep.EmptyBlocks)
		}
		if len(rep.Functions) != 1 || rep.Functions[0].LOC != 1 || !rep.Functions[0].EmptyBlock {
			t.Errorf("expected empty block function metric, got: %+v", rep.Functions)
		}
	})

	t.Run("VEC-TS-08 Markdown Structural Outline", func(t *testing.T) {
		src := []byte("# Section\n## Sub\nParagraph")
		rep, err := engine.ParseAndEvaluate(ctx, "spec.md", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !rep.Passed {
			t.Errorf("expected markdown outline parse to pass")
		}
		if !rep.OutlineVerified {
			t.Errorf("expected outline to be verified")
		}
		if len(rep.HeadingLevels) != 2 || rep.HeadingLevels[0] != 1 || rep.HeadingLevels[1] != 2 {
			t.Errorf("expected heading levels [1, 2], got: %+v", rep.HeadingLevels)
		}
	})

	t.Run("VEC-TS-09 Svelte Script Extraction", func(t *testing.T) {
		src := []byte("<script lang=\"ts\">\n  let count = 0;\n  function increment() {\n    if (count < 10) count++;\n  }\n</script>\n\n<button on:click={increment}>{count}</button>")
		rep, err := engine.ParseAndEvaluate(ctx, "Counter.svelte", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rep.Functions) != 1 {
			t.Fatalf("expected 1 function in svelte script, got %d", len(rep.Functions))
		}
		fn := rep.Functions[0]
		if fn.Name != "increment" || fn.Complexity != 2 || fn.Line != 3 {
			t.Errorf("unexpected svelte function metrics: %+v", fn)
		}
	})

	t.Run("VEC-TS-10 Wazero WebAssembly Runtime Wire", func(t *testing.T) {
		if engine.Runtime() == nil {
			t.Fatalf("expected non-nil wazero runtime")
		}
		if engine.Module() == nil {
			t.Fatalf("expected non-nil instantiated wasm module")
		}
		if engine.CompiledModule() == nil {
			t.Fatalf("expected non-nil compiled wasm module")
		}
		if err := engine.ExecuteWasmVerify(ctx); err != nil {
			t.Errorf("wasm verify execution failed: %v", err)
		}
	})
}

func TestGetDefaultEngine(t *testing.T) {
	eng1, err := GetDefaultEngine()
	if err != nil {
		t.Fatalf("failed getting default engine: %v", err)
	}
	eng2, err := GetDefaultEngine()
	if err != nil {
		t.Fatalf("failed getting second default engine: %v", err)
	}
	if eng1 != eng2 {
		t.Errorf("expected singleton engine instance")
	}
}

