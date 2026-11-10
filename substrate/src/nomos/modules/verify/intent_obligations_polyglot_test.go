package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func seedPolyglotTestFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	tmp := t.TempDir()
	specDir := filepath.Join(tmp, "specs")
	tsDir := filepath.Join(tmp, "src", "billing")
	pyDir := filepath.Join(tmp, "app", "ledger")
	rbSpecDir := filepath.Join(tmp, "spec")
	for _, d := range []string{specDir, tsDir, pyDir, rbSpecDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	nomosYAML := "project: polyglot-demo\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: true\n    intent_source_roots:\n      - \"specs\"\n    contracts:\n      contracts:\n        enabled: true\n        spec_roots:\n          - \"specs\"\n        strict_types: true\n        verify_obligations: true\n      infra:\n        enabled: true\n        fail_on_orphan_env: true\n        env_declarations:\n          - \".env.example\"\n    thresholds:\n      max_cognitive_debt_score: 45\n      max_cyclomatic_complexity: 15\n      max_file_loc: 400\n      max_func_loc: 60\n      min_comment_density_pct: 10\n      min_test_coverage_pct: 55\n"
	mustWritePolyglotFile(t, filepath.Join(tmp, "nomos.yaml"), nomosYAML)
	mustWritePolyglotFile(t, filepath.Join(tmp, ".env.example"), "STRIPE_PUBLIC_KEY=pk_test\n")

	specMD := "# Billing Spec\n\n```typescript\nexport interface InvoicePayload {\n  id: string;\n  currency: string;\n}\n```\n\n```python\nclass LedgerEntry:\n    entry_id: str\n    amount_cents: int\n```\n\n| Vector | Check |\n| :--- | :--- |\n| `TC-BIL-01` | Ruby spec binding |\n"
	mustWritePolyglotFile(t, filepath.Join(specDir, "billing_spec.md"), specMD)
	return tmp, tsDir, pyDir, rbSpecDir
}

func mustWritePolyglotFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestPolyglotSpecSignaturesTraceabilityAndEnvGates(t *testing.T) {
	tmp, tsDir, pyDir, rbSpecDir := seedPolyglotTestFixture(t)
	mustWritePolyglotFile(t, filepath.Join(tsDir, "invoice.ts"), "// @intent billing_spec.md #GhostType\nexport interface InvoicePayload {\n  id: string;\n}\nexport const secret = process.env.UNDECLARED_BILLING_SECRET;\n")
	mustWritePolyglotFile(t, filepath.Join(pyDir, "entry.py"), "# @intent billing_spec.md #LedgerEntry\nclass LedgerEntry:\n    entry_id: str\n")

	t.Run("RejectsDriftedFieldsAndOrphanEnv", func(t *testing.T) {
		assertPolyglotDriftAndOrphanEnvRejected(t, tmp)
	})
	t.Run("PassesAlignedContractsAndRubyRSpecBinding", func(t *testing.T) {
		assertPolyglotAlignedAndRSpecBound(t, tmp, tsDir, pyDir, rbSpecDir)
	})
	t.Run("RecognizesAllElevenLanguageTestConventions", func(t *testing.T) {
		for _, sample := range []string{
			"order_test.go", "test_order.py", "order_test.py", "order.test.ts", "order.spec.tsx",
			"order_spec.rb", "OrderTest.java", "OrderTests.kt", "OrderTests.cs", "order_test.cpp",
		} {
			if !isVerificationSuiteFile(sample) {
				t.Fatalf("expected %s recognized as verification suite file", sample)
			}
		}
	})
}

func assertPolyglotDriftAndOrphanEnvRejected(t *testing.T, tmp string) {
	t.Helper()
	gate := &IntentTraceabilityGate{}
	ctx := &VerifyContext{RepoRoot: tmp, WorktreePath: tmp}
	resSig, err := gate.Evaluate(ctx)
	if err != nil || resSig == nil || resSig.Passed || !strings.Contains(resSig.Message, "currency") || !strings.Contains(resSig.Message, "amount_cents") {
		t.Fatalf("expected polyglot TS + Python field drift rejected, got: %+v, err=%v", resSig, err)
	}
	intRes, iErr := gate.Evaluate(&VerifyContext{RepoRoot: tmp, WorktreePath: tmp, TaskID: "INT-55"})
	if iErr != nil || intRes == nil || intRes.Passed || !strings.Contains(intRes.Message, "#GhostType") {
		t.Fatalf("expected INT-55 to skip SUB obligation checks and catch #GhostType, got: %+v, err=%v", intRes, iErr)
	}
	infraGate := &InfraPortIngressGate{}
	infraRes, infErr := infraGate.Evaluate(ctx)
	if infErr != nil || infraRes == nil || infraRes.Passed || !strings.Contains(infraRes.Message, "UNDECLARED_BILLING_SECRET") {
		t.Fatalf("expected InfraPortIngressGate to catch undeclared TS process.env var, got: %+v, err=%v", infraRes, infErr)
	}
}

func assertPolyglotAlignedAndRSpecBound(t *testing.T, tmp, tsDir, pyDir, rbSpecDir string) {
	t.Helper()
	mustWritePolyglotFile(t, filepath.Join(tsDir, "invoice.ts"), "// @intent billing_spec.md #InvoicePayload\nexport interface InvoicePayload {\n  id: string;\n  currency: string;\n}\nexport const pubKey = process.env.STRIPE_PUBLIC_KEY;\n")
	mustWritePolyglotFile(t, filepath.Join(pyDir, "entry.py"), "# @intent billing_spec.md #LedgerEntry\nclass LedgerEntry:\n    entry_id: str\n    amount_cents: int\n")
	mustWritePolyglotFile(t, filepath.Join(rbSpecDir, "billing_spec.rb"), "# @verifies TC-BIL-01\nRSpec.describe 'Billing' do\n  it 'validates invoice' do\n    expect(1).to eq(1)\n  end\nend\n")

	ctx := &VerifyContext{RepoRoot: tmp, WorktreePath: tmp}
	gate := &IntentTraceabilityGate{}
	if resPass, err := gate.Evaluate(ctx); err != nil || resPass == nil || !resPass.Passed {
		t.Fatalf("expected polyglot IntentTraceabilityGate to pass cleanly, got: %+v, err=%v", resPass, err)
	}
	infraGate := &InfraPortIngressGate{}
	if infraPass, err := infraGate.Evaluate(ctx); err != nil || infraPass == nil || !infraPass.Passed {
		t.Fatalf("expected polyglot InfraPortIngressGate to pass cleanly, got: %+v, err=%v", infraPass, err)
	}
}
