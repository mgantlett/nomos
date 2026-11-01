package treesitter

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestASTMutationTesting_FaultInjection validates that Tree-sitter quality gates
// are sensitive to synthetic faults (empty blocks, complexity spikes, LOC overruns, syntax corruption).
func TestASTMutationTesting_FaultInjection(t *testing.T) {
	ctx := context.Background()
	engine, err := NewTreeSitterEngine(ctx)
	if err != nil {
		t.Fatalf("failed to initialize treesitter engine: %v", err)
	}
	defer engine.Close(ctx)

	t.Run("Fault-1: Empty Block Injection Rejection", func(t *testing.T) {
		languages := []struct {
			filename string
			valid    string
			mutated  string
		}{
			{
				filename: "service.go",
				valid:    "// Run starts service\nfunc Run() error {\n\treturn start()\n}",
				mutated:  "// Run starts service\nfunc Run() {}",
			},
			{
				filename: "worker.rs",
				valid:    "/// Process jobs\nfn process() {\n\texecute();\n}",
				mutated:  "/// Process jobs\nfn process() {}",
			},
			{
				filename: "handler.ts",
				valid:    "/** Handle request */\nexport function handle() {\n\treturn 1;\n}",
				mutated:  "/** Handle request */\nexport function handle() {}",
			},
		}

		for _, tc := range languages {
			t.Run(tc.filename, func(t *testing.T) {
				// Base valid passes
				repValid, err := engine.ParseAndEvaluate(ctx, tc.filename, []byte(tc.valid))
				if err != nil {
					t.Fatalf("valid source failed: %v", err)
				}
				if !repValid.Passed {
					t.Errorf("expected valid source to pass, got: %+v", repValid)
				}

				// Mutated empty block fails closed
				repMut, err := engine.ParseAndEvaluate(ctx, tc.filename, []byte(tc.mutated))
				if err != nil {
					t.Fatalf("mutated parse failed unexpectedly: %v", err)
				}
				if repMut.Passed {
					t.Errorf("expected mutated empty block in %s to fail gate evaluation", tc.filename)
				}
				if len(repMut.EmptyBlocks) == 0 {
					t.Errorf("expected empty block finding in %s", tc.filename)
				}
			})
		}
	})

	t.Run("Fault-2: Cyclomatic Complexity Overrun Injection", func(t *testing.T) {
		// Inject 16 if conditions into a function to exceed max complexity threshold of 15
		var b strings.Builder
		b.WriteString("// Overcomplex function\nfunc Overcomplex(x int) {\n")
		for i := 0; i < 16; i++ {
			b.WriteString(fmt.Sprintf("\tif x == %d { return }\n", i))
		}
		b.WriteString("}\n")

		rep, err := engine.ParseAndEvaluate(ctx, "complex.go", []byte(b.String()))
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if rep.Passed {
			t.Errorf("expected complexity overrun to fail evaluation")
		}
		if len(rep.Functions) == 0 || rep.Functions[0].Complexity <= 15 {
			t.Errorf("expected function complexity > 15, got: %+v", rep.Functions)
		}
	})

	t.Run("Fault-3: Function LOC Overrun Injection", func(t *testing.T) {
		// Default threshold max func LOC is 80. Inject 90 lines.
		var b strings.Builder
		b.WriteString("// HugeFunc is too long\nfunc HugeFunc() {\n")
		for i := 0; i < 85; i++ {
			b.WriteString(fmt.Sprintf("\t_ = %d\n", i))
		}
		b.WriteString("}\n")

		rep, err := engine.ParseAndEvaluate(ctx, "huge.go", []byte(b.String()))
		if err != nil {
			t.Fatalf("parse failed: %v", err)
		}
		if rep.Passed {
			t.Errorf("expected LOC overrun to fail evaluation")
		}
		if len(rep.Functions) == 0 || rep.Functions[0].LOC <= 80 {
			t.Errorf("expected function LOC > 80, got: %+v", rep.Functions)
		}
	})

	t.Run("Fault-4: Syntax Corruption Fail-Closed", func(t *testing.T) {
		corruptedSources := []struct {
			filename string
			src      string
		}{
			{"bad.go", "func Malformed( { ERROR_SYNTAX"},
			{"bad.rs", "fn malformed( { ERROR_SYNTAX"},
			{"bad.py", "def malformed( : ERROR_SYNTAX\n\t..."},
			{"bad.ts", "function malformed( { ERROR_SYNTAX"},
			{"bad.html", "<div>< unclosed ERROR_SYNTAX"},
			{"bad.css", "body { color: ERROR_SYNTAX"},
		}

		for _, tc := range corruptedSources {
			t.Run(tc.filename, func(t *testing.T) {
				_, err := engine.ParseAndEvaluate(ctx, tc.filename, []byte(tc.src))
				if err == nil {
					t.Errorf("expected syntax corruption in %s to fail closed with error", tc.filename)
				}
			})
		}
	})
}
