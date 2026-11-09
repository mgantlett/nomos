package treesitter

import (
	"context"
	"strings"
	"testing"
)

func newTestTreeSitterEngine(t *testing.T) (context.Context, *WazeroTreeSitterEngine) {
	t.Helper()
	ctx := context.Background()
	engine, err := NewTreeSitterEngine(ctx)
	if err != nil {
		t.Fatalf("failed to initialize treesitter engine: %v", err)
	}
	t.Cleanup(func() { engine.Close(ctx) })
	return ctx, engine
}

func TestTreeSitterEngine_CoreLanguageVectors(t *testing.T) {
	ctx, engine := newTestTreeSitterEngine(t)

	t.Run("VEC-TS-01 Go Valid Doc and Branch", func(t *testing.T) {
		src := []byte("// Exec runs task\nfunc Exec() error {\n\tif err := step(); err != nil {\n\t\treturn err\n\t}\n\treturn nil\n}")
		rep, err := engine.ParseAndEvaluate(ctx, "exec.go", src)
		if err != nil || !rep.Passed || len(rep.Functions) != 1 {
			t.Fatalf("unexpected report: err=%v rep=%+v", err, rep)
		}
		fn := rep.Functions[0]
		if fn.Name != "Exec" || !fn.Documented || fn.Complexity != 2 {
			t.Errorf("unexpected metrics: %+v", fn)
		}
	})

	t.Run("VEC-TS-02 Go Syntax Error Fail Closed", func(t *testing.T) {
		if _, err := engine.ParseAndEvaluate(ctx, "err.go", []byte("func Incomplete() { ERROR_SYNTAX")); err == nil {
			t.Errorf("expected fail-closed error on syntax error, got nil")
		}
	})

	t.Run("VEC-TS-03 TypeScript Arrow Function", func(t *testing.T) {
		src := []byte("export const handle = (req: Req) => {\n\tif (a && b || c) {\n\t\treturn d;\n\t}\n};")
		rep, err := engine.ParseAndEvaluate(ctx, "handler.ts", src)
		if err != nil || len(rep.Functions) != 1 {
			t.Fatalf("unexpected report: err=%v rep=%+v", err, rep)
		}
		if rep.Functions[0].Name != "handle" || rep.Functions[0].Complexity < 4 {
			t.Errorf("expected handle with complexity >= 4, got: %+v", rep.Functions[0])
		}
	})

	t.Run("VEC-TS-04 Python Branches", func(t *testing.T) {
		src := []byte("def dispatch(event):\n    if event:\n        for item in event:\n            pass")
		rep, err := engine.ParseAndEvaluate(ctx, "event.py", src)
		if err != nil || len(rep.Functions) != 1 {
			t.Fatalf("unexpected report: err=%v rep=%+v", err, rep)
		}
		if rep.Functions[0].Name != "dispatch" || rep.Functions[0].Complexity != 3 {
			t.Errorf("expected dispatch with complexity 3, got: %+v", rep.Functions[0])
		}
	})
}

func TestTreeSitterEngine_StructuralAndWasmVectors(t *testing.T) {
	ctx, engine := newTestTreeSitterEngine(t)

	t.Run("VEC-TS-05 Go Complexity Exceeded", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("package test\nfunc Complex(x int) {\n")
		for i := 0; i < 16; i++ {
			b.WriteString("\tif x == 1 { return }\n")
		}
		b.WriteString("}\n")
		rep, err := engine.ParseAndEvaluate(ctx, "complex.go", []byte(b.String()))
		if err != nil || rep.Passed || rep.Functions[0].Complexity != 17 {
			t.Fatalf("expected complexity 17 failure, got err=%v rep=%+v", err, rep)
		}
	})

	t.Run("VEC-TS-06 TypeScript JSDoc Adjacency", func(t *testing.T) {
		src := []byte("/** Valid JSDoc */\nfunction calc(): void {\n\treturn;\n}")
		rep, err := engine.ParseAndEvaluate(ctx, "calc.ts", src)
		if err != nil || len(rep.Functions) != 1 || !rep.Functions[0].Documented {
			t.Errorf("expected documented function, got err=%v fns=%+v", err, rep)
		}
	})

	t.Run("VEC-TS-07 Go Empty Block Shim Violation", func(t *testing.T) {
		rep, err := engine.ParseAndEvaluate(ctx, "shim.go", []byte("package test\nfunc Shim() {}"))
		if err != nil || rep.Passed || len(rep.EmptyBlocks) != 1 || rep.EmptyBlocks[0] != "Shim" {
			t.Fatalf("expected empty block Shim failure, got err=%v rep=%+v", err, rep)
		}
		if len(rep.Functions) != 1 || rep.Functions[0].LOC != 1 || !rep.Functions[0].EmptyBlock {
			t.Errorf("expected empty block function metric, got: %+v", rep.Functions)
		}
	})

	t.Run("VEC-TS-08 Markdown Structural Outline", func(t *testing.T) {
		rep, err := engine.ParseAndEvaluate(ctx, "spec.md", []byte("# Section\n## Sub\nParagraph"))
		if err != nil || !rep.Passed || !rep.OutlineVerified {
			t.Fatalf("expected markdown outline pass, got err=%v rep=%+v", err, rep)
		}
		if len(rep.HeadingLevels) != 2 || rep.HeadingLevels[0] != 1 || rep.HeadingLevels[1] != 2 {
			t.Errorf("expected heading levels [1, 2], got: %+v", rep.HeadingLevels)
		}
	})

	t.Run("VEC-TS-10 Wazero WebAssembly Runtime Wire", func(t *testing.T) {
		if engine.Runtime() == nil || engine.Module() == nil || engine.CompiledModule() == nil {
			t.Fatalf("expected non-nil wazero runtime and wasm modules")
		}
		if err := engine.ExecuteWasmVerify(ctx); err != nil {
			t.Errorf("wasm verify execution failed: %v", err)
		}
	})
}

func TestTreeSitterEngine_PolyglotShimAndSkipVectors(t *testing.T) {
	ctx, engine := newTestTreeSitterEngine(t)

	t.Run("VEC-TS-09 Svelte and Template Literal Isolation", func(t *testing.T) {
		svelteSrc := []byte("<script lang=\"ts\">\n  let count = 0;\n  function increment() {\n    if (count < 10) count++;\n  }\n</script>\n\n<button on:click={increment}>{count}</button>")
		repSv, err := engine.ParseAndEvaluate(ctx, "Counter.svelte", svelteSrc)
		if err != nil || len(repSv.Functions) != 1 || repSv.Functions[0].Name != "increment" || repSv.Functions[0].Complexity != 2 || repSv.Functions[0].Line != 3 {
			t.Fatalf("unexpected svelte report: err=%v rep=%+v", err, repSv)
		}

		tplSrc := []byte("/** Render template */\nexport function renderCard(active: boolean): string {\n\tconst tpl = `\n\t\tfunction fakeInner() {\n\t\t\tif (a && b || c && d) { return; }\n\t\t}\n\t`;\n\tif (active) {\n\t\treturn tpl;\n\t}\n\treturn \"\";\n}\n")
		repTpl, err := engine.ParseAndEvaluate(ctx, "card.ts", tplSrc)
		if err != nil || len(repTpl.Functions) != 1 || repTpl.Functions[0].Name != "renderCard" || repTpl.Functions[0].Complexity != 2 {
			t.Fatalf("unexpected template literal report: err=%v rep=%+v", err, repTpl)
		}
	})

	t.Run("VEC-TS-12 JVM and CSharp AST Evaluation and AntiShim", func(t *testing.T) {
		javaSrc := []byte("public class Engine {\n\t/** Execute step */\n\tpublic int executeStep(int x) {\n\t\tif (x > 0 && x < 10) {\n\t\t\treturn x;\n\t\t}\n\t\treturn 0;\n\t}\n}\n")
		repJava, err := engine.ParseAndEvaluate(ctx, "Engine.java", javaSrc)
		if err != nil || !repJava.Passed || len(repJava.Functions) != 1 || repJava.Functions[0].Complexity != 3 {
			t.Fatalf("unexpected java report: err=%v rep=%+v", err, repJava)
		}
		for file, code := range map[string]string{
			"Worker.kt":  "class Worker {\n\tfun runTask() {\n\t\tTODO(\"pending\")\n\t}\n}\n",
			"Handler.cs": "public class Handler {\n\tpublic void Handle() {\n\t\tthrow new NotImplementedException();\n\t}\n}\n",
		} {
			rep, sErr := engine.ParseAndEvaluate(ctx, file, []byte(code))
			if sErr != nil || rep.Passed || len(rep.EmptyBlocks) != 1 {
				t.Fatalf("expected %s shim to fail closed: err=%v rep=%+v", file, sErr, rep)
			}
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
			if err != nil || rep.Passed || len(rep.SkippedTests) == 0 {
				t.Errorf("expected skipped test in %s to fail closed, got err=%v Passed=%v SkippedTests=%v", file, err, rep.Passed, rep.SkippedTests)
			}
		}
	})
}

func TestTreeSitterEngine_RubyAndSingletonVectors(t *testing.T) {
	ctx, engine := newTestTreeSitterEngine(t)

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
	if err != nil || !repRb.Passed || len(repRb.Functions) != 2 {
		t.Fatalf("expected 2 passing Ruby methods, got err=%v rep=%+v", err, repRb)
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

	eng1, err := GetDefaultEngine()
	if err != nil {
		t.Fatalf("failed getting default engine: %v", err)
	}
	eng2, err := GetDefaultEngine()
	if err != nil || eng1 != eng2 {
		t.Fatalf("expected singleton engine instance, err=%v", err)
	}
}
