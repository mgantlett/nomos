package treesitter

import (
	"context"
	"testing"
)

func TestPolyglotParity_LiteralStrippingNestedSpansAndMultilineHeaders(t *testing.T) {
	ctx := context.Background()
	engine, err := GetDefaultEngine()
	if err != nil {
		t.Fatalf("GetDefaultEngine: %v", err)
	}

	t.Run("Python docstring and SQL string keywords do not inflate complexity", func(t *testing.T) {
		pySrc := []byte(`def build_query(active):
    """
    Returns query if active and ready or fallback for every row while scanning.
    def fake_inside_docstring():
        if a and b or c:
            pass
    """
    sql = "SELECT * FROM items WHERE a = 1 AND b = 2 OR c = 3 -- if for while and or"
    if active:
        return sql
    return ""
`)
		rep, pErr := engine.ParseAndEvaluate(ctx, "query.py", pySrc)
		if pErr != nil {
			t.Fatalf("unexpected python parse error: %v", pErr)
		}
		if len(rep.Functions) != 1 {
			t.Fatalf("expected 1 function, got %d: %+v", len(rep.Functions), rep.Functions)
		}
		if rep.Functions[0].Complexity != 2 || !rep.Functions[0].Documented {
			t.Errorf("expected complexity 2 and Documented=true, got %+v", rep.Functions[0])
		}
	})

	t.Run("Nested functions do not double-count complexity across Python TS and Rust", func(t *testing.T) {
		nestedPy := []byte(`def outer(items):
    """Outer handler."""
    def inner(val):
        if val > 0 and val < 10 or val == 99:
            return val
        return 0
    if items:
        return [inner(x) for x in items]
    return []
`)
		repPy, err := engine.ParseAndEvaluate(ctx, "nested.py", nestedPy)
		if err != nil || len(repPy.Functions) != 2 {
			t.Fatalf("expected 2 python functions, err=%v funcs=%+v", err, repPy)
		}
		// outer has 1 + if (1) + comprehension for (1) = 3; inner has 1 + if + and + or = 4
		if repPy.Functions[0].Name != "outer" || repPy.Functions[0].Complexity != 3 {
			t.Errorf("expected outer complexity 3 (without inner's branches), got %+v", repPy.Functions[0])
		}
		if repPy.Functions[1].Name != "inner" || repPy.Functions[1].Complexity != 4 {
			t.Errorf("expected inner complexity 4, got %+v", repPy.Functions[1])
		}

		nestedTS := []byte(`/** Outer TS */
export function processBatch(
	items: number[],
	scale: number,
): number {
	function helper(v: number): number {
		if (v > 0 && v < 10 || v === 42) {
			return v;
		}
		return 0;
	}
	if (scale > 0) {
		return helper(items.length);
	}
	return 0;
}
`)
		repTS, tsErr := engine.ParseAndEvaluate(ctx, "batch.ts", nestedTS)
		if tsErr != nil || len(repTS.Functions) != 2 {
			t.Fatalf("expected 2 TS functions with multiline signature, err=%v funcs=%+v", tsErr, repTS)
		}
		if repTS.Functions[0].Name != "processBatch" || repTS.Functions[0].Complexity != 2 {
			t.Errorf("expected outer TS processBatch complexity 2, got %+v", repTS.Functions[0])
		}
		if repTS.Functions[1].Name != "helper" || repTS.Functions[1].Complexity != 4 {
			t.Errorf("expected inner TS helper complexity 4, got %+v", repTS.Functions[1])
		}

		nestedRust := []byte(`/// Compute batch total
pub fn compute_batch(
	count: i32,
	enabled: bool,
) -> i32 {
	fn clamp(v: i32) -> i32 {
		if v > 0 && v < 100 {
			return v;
		}
		0
	}
	if enabled {
		return clamp(count);
	}
	0
}
`)
		repRs, rsErr := engine.ParseAndEvaluate(ctx, "batch.rs", nestedRust)
		if rsErr != nil || len(repRs.Functions) != 2 {
			t.Fatalf("expected 2 Rust functions, err=%v funcs=%+v", rsErr, repRs)
		}
		if repRs.Functions[0].Name != "compute_batch" || repRs.Functions[0].Complexity != 2 {
			t.Errorf("expected outer Rust compute_batch complexity 2, got %+v", repRs.Functions[0])
		}
		if repRs.Functions[1].Name != "clamp" || repRs.Functions[1].Complexity != 3 {
			t.Errorf("expected inner Rust clamp complexity 3, got %+v", repRs.Functions[1])
		}
	})
}

