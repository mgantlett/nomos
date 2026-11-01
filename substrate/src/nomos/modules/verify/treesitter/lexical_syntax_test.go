package treesitter

import (
	"context"
	"testing"
)

func TestPolyglotRealSyntaxAndAsyncPython(t *testing.T) {
	ctx := context.Background()
	engine, err := GetDefaultEngine()
	if err != nil {
		t.Fatalf("failed acquiring engine: %v", err)
	}

	t.Run("Real Syntax Errors Fail Closed Without Sentinel String", func(t *testing.T) {
		broken := map[string]string{
			"unclosed.rs":   "pub fn run(flag: bool) {\n    if flag {\n        return;\n",
			"unclosed.c":    "int calc(int x) {\n    return x + 1;\n",
			"unclosed.py":   "async def fetch(url: str,\n    timeout: int = 5\n    return None\n",
			"unclosed.ts":   "export function load(id: string {\n    return id;\n}",
			"unclosed.html": "<!DOCTYPE html><html><body><div>missing close</body></html>",
			"unclosed.css":  ".card {\n  display: flex;\n",
		}
		for name, src := range broken {
			if _, parseErr := engine.ParseAndEvaluate(ctx, name, []byte(src)); parseErr == nil {
				t.Errorf("expected real syntax error in %s to fail closed", name)
			}
		}
	})

	t.Run("Python Async Def and Multiline Signature Extraction", func(t *testing.T) {
		pySrc := `async def fetch_records(
    endpoint: str,
    limit: int = 25,
) -> list:
    """Fetch bounded records from endpoint."""
    if not endpoint:
        return []
    return [endpoint]
`
		rep, err := engine.ParseAndEvaluate(ctx, "client.py", []byte(pySrc))
		if err != nil {
			t.Fatalf("unexpected python parse error: %v", err)
		}
		if !rep.Passed || len(rep.Functions) != 1 {
			t.Fatalf("expected 1 passing python async function, got %+v", rep)
		}
		fn := rep.Functions[0]
		if fn.Name != "fetch_records" || !fn.Documented || fn.Complexity != 2 || fn.EmptyBlock {
			t.Errorf("unexpected async python metrics: %+v", fn)
		}
	})

	t.Run("Rust Lifetimes and Braces in Strings Ignored Cleanly", func(t *testing.T) {
		rsSrc := `/// Borrowed parser with lifetime and brace literal
pub fn token_kind<'a>(input: &'a str, ch: char) -> &'a str {
    let sample = "{ unclosed brace inside string literal (";
    if ch == '{' && !sample.is_empty() {
        return input;
    }
    input
}
`
		rep, err := engine.ParseAndEvaluate(ctx, "lexer.rs", []byte(rsSrc))
		if err != nil {
			t.Fatalf("unexpected rust parse error on lifetime/string braces: %v", err)
		}
		if !rep.Passed || len(rep.Functions) != 1 || rep.Functions[0].Name != "token_kind" {
			t.Errorf("unexpected rust report: %+v", rep)
		}
	})
}
