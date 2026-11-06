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

	t.Run("VEC-TS-11 Multiline Template Literal Differential Isolation", func(t *testing.T) {
		src := []byte("/** Render template */\nexport function renderCard(active: boolean): string {\n\tconst tpl = `\n\t\tfunction fakeInner() {\n\t\t\tif (a && b || c && d) { return; }\n\t\t}\n\t`;\n\tif (active) {\n\t\treturn tpl;\n\t}\n\treturn \"\";\n}\n")
		rep, err := engine.ParseAndEvaluate(ctx, "card.ts", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rep.Functions) != 1 {
			t.Fatalf("expected exactly 1 function (no false positive from template literal), got %d: %+v", len(rep.Functions), rep.Functions)
		}
		if rep.Functions[0].Name != "renderCard" || rep.Functions[0].Complexity != 2 {
			t.Errorf("expected renderCard with complexity 2, got: %+v", rep.Functions[0])
		}
	})

	t.Run("VEC-TS-12 JVM and CSharp AST Evaluation and AntiShim", func(t *testing.T) {
		javaSrc := []byte("public class Engine {\n\t/** Execute step */\n\tpublic int executeStep(int x) {\n\t\tif (x > 0 && x < 10) {\n\t\t\treturn x;\n\t\t}\n\t\treturn 0;\n\t}\n}\n")
		repJava, err := engine.ParseAndEvaluate(ctx, "Engine.java", javaSrc)
		if err != nil || !repJava.Passed || len(repJava.Functions) != 1 || repJava.Functions[0].Complexity != 3 {
			t.Fatalf("unexpected java report: err=%v rep=%+v", err, repJava)
		}

		ktShim := []byte("class Worker {\n\tfun runTask() {\n\t\tTODO(\"pending\")\n\t}\n}\n")
		repKt, err := engine.ParseAndEvaluate(ctx, "Worker.kt", ktShim)
		if err != nil || repKt.Passed || len(repKt.EmptyBlocks) != 1 {
			t.Fatalf("expected kotlin TODO() shim to fail closed: err=%v rep=%+v", err, repKt)
		}

		csShim := []byte("public class Handler {\n\tpublic void Handle() {\n\t\tthrow new NotImplementedException();\n\t}\n}\n")
		repCS, err := engine.ParseAndEvaluate(ctx, "Handler.cs", csShim)
		if err != nil || repCS.Passed || len(repCS.EmptyBlocks) != 1 {
			t.Fatalf("expected csharp NotImplementedException shim to fail closed: err=%v rep=%+v", err, repCS)
		}
	})

	t.Run("VEC-TS-13 Test Skip Reward Hack Rejection", func(t *testing.T) {
		cases := map[string]string{
			"app.test.ts":   "it.skip('bypassesbroken test', () => {});",
			"test_app.py":   "@pytest.mark.skip\ndef test_broken():\n    assert True\n",
			"order_spec.rb": "RSpec.describe Order do\n  xit 'bypasses broken spec' do\n    expect(true).to be(true)\n  end\nend\n",
			"order_test.rb": "class OrderTest < ActiveSupport::TestCase\n  def test_broken\n    skip 'temporarily muted'\n  end\nend\n",
			"lib_test.rs":   "#[ignore]\nfn test_broken() {\n    assert!(true);\n}\n",
			"AppTest.java":  "public class AppTest {\n\t@Disabled\n\tpublic void testBroken() {\n\t\tassertTrue(true);\n\t}\n}\n",
			"AppTests.cs":   "public class AppTests {\n\t[Fact(Skip = \"broken\")]\n\tpublic void TestBroken() {\n\t\tAssert.True(true);\n\t}\n}\n",
		}
		for file, code := range cases {
			rep, err := engine.ParseAndEvaluate(ctx, file, []byte(code))
			if err != nil {
				t.Fatalf("unexpected parse err on %s: %v", file, err)
			}
			if rep.Passed || len(rep.SkippedTests) == 0 {
				t.Errorf("expected skipped test in %s to fail closed, got Passed=%v SkippedTests=%v", file, rep.Passed, rep.SkippedTests)
			}
		}
	})

	t.Run("VEC-TS-14 Ruby and Rails Idioms Heredoc Endless and AntiShim", func(t *testing.T) {
		railsModel := []byte(`class Order < ApplicationRecord
  # Compute settled cents when valid
  def settled_cents
    sql = <<~SQL
      SELECT CASE WHEN paid THEN amount ELSE 0 END FROM orders
    SQL
    return 0 unless sql && valid_amount?
    amount_cents
  end

  # Predicate using Ruby 3 endless method syntax
  def valid_amount? = amount_cents > 0 && !refunded?
end
`)
		repRb, err := engine.ParseAndEvaluate(ctx, "app/models/order.rb", railsModel)
		if err != nil {
			t.Fatalf("unexpected ruby parse error: %v", err)
		}
		if !repRb.Passed || len(repRb.Functions) != 2 {
			t.Fatalf("expected 2 passing Ruby methods, got Passed=%v Functions=%+v", repRb.Passed, repRb.Functions)
		}
		if repRb.Functions[0].Name != "settled_cents" || !repRb.Functions[0].Documented || repRb.Functions[0].Complexity != 3 {
			t.Errorf("unexpected settled_cents metrics: %+v", repRb.Functions[0])
		}
		if repRb.Functions[1].Name != "valid_amount?" || repRb.Functions[1].LOC != 1 || repRb.Functions[1].Complexity != 2 {
			t.Errorf("unexpected endless method metrics: %+v", repRb.Functions[1])
		}

		rbShim := []byte("class PaymentGateway\n  def charge!(cents)\n    raise NotImplementedError, 'pending'\n  end\nend\n")
		repShim, sErr := engine.ParseAndEvaluate(ctx, "app/models/payment_gateway.rb", rbShim)
		if sErr != nil || repShim.Passed || len(repShim.EmptyBlocks) != 1 {
			t.Fatalf("expected Ruby NotImplementedError shim to fail closed: err=%v rep=%+v", sErr, repShim)
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

