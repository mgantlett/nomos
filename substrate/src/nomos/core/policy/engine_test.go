package policy

import (
	"context"
	"testing"
)

func TestCUEEngine_Validate(t *testing.T) {
	engine := NewCUEEngine()

	schema := `
package test
#Config: {
	name: string
	port: int & > 0 & < 65536
}
#Config
`

	validData := map[string]any{
		"name": "nomos",
		"port": 8080,
	}

	if err := engine.Validate(schema, validData); err != nil {
		t.Fatalf("expected valid data to pass validation, got: %v", err)
	}

	invalidData := map[string]any{
		"name": "nomos",
		"port": 99999,
	}

	if err := engine.Validate(schema, invalidData); err == nil {
		t.Fatalf("expected port out of range to fail validation")
	}
}

func TestCUEEngine_EvaluateBool(t *testing.T) {
	engine := NewCUEEngine()

	schema := `
package test
min_score: int | *80
score: int | *0
passed: score >= min_score
`

	ctx := context.Background()

	res, err := engine.EvaluateBool(ctx, schema, "passed", map[string]any{"score": 90})
	if err != nil {
		t.Fatalf("unexpected error evaluating bool: %v", err)
	}
	if !res {
		t.Errorf("expected score 90 >= 80 to evaluate to true")
	}

	res, err = engine.EvaluateBool(ctx, schema, "passed", map[string]any{"score": 70})
	if err != nil {
		t.Fatalf("unexpected error evaluating bool: %v", err)
	}
	if res {
		t.Errorf("expected score 70 >= 80 to evaluate to false")
	}
}

func TestCUEEngine_EvaluateViolations(t *testing.T) {
	engine := NewCUEEngine()

	schema := `
package test

files: [...{name: string, cds: int}]
threshold: int | *45

cds_violations: [
	for f in files if f.cds > threshold {
		"\(f.name) exceeds max cds \(threshold)"
	}
]
`

	ctx := context.Background()
	input := map[string]any{
		"threshold": 45,
		"files": []map[string]any{
			{"name": "clean.go", "cds": 20},
			{"name": "debt.go", "cds": 60},
		},
	}

	violations, err := engine.EvaluateViolations(ctx, schema, "cds_violations", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0] != "debt.go exceeds max cds 45" {
		t.Errorf("unexpected violation string: %s", violations[0])
	}
}
