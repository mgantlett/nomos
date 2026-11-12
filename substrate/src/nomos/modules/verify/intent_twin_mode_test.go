package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

func TestValidateCommitIntentTwinMode_OffFeatStrict(t *testing.T) {
	subFeatUntwinned := `[SUB-42] feat(cli): add container sandbox mode

Add disposable container sandbox flag for benchmark trials.

___

Task: SUB-42`

	subFeatTwinned := subFeatUntwinned + "\nTwin: INT-21"

	subFixUntwinned := `[SUB-43] fix(cli): trim trailing whitespace

Normalize trailing newline handling during sync.

___

Task: SUB-43`

	intDocsUntwinned := `[INT-21] docs(specs): codify container sandbox mode

Document --container-sandbox in indirect control spec.

___

Task: INT-21`

	if v := ValidateCommitIntentTwinMode(subFeatUntwinned, config.IntentTwinModeOff); v != "" {
		t.Fatalf("expected off mode to allow untwinned SUB feat, got %q", v)
	}
	if v := ValidateCommitIntentTwinMode(subFeatUntwinned, config.IntentTwinModeFeat); v == "" {
		t.Fatal("expected feat mode to reject untwinned SUB feat commit")
	}
	if v := ValidateCommitIntentTwinMode(subFeatTwinned, config.IntentTwinModeFeat); v != "" {
		t.Fatalf("expected feat mode to allow twinned SUB feat commit, got %q", v)
	}
	if v := ValidateCommitIntentTwinMode(subFixUntwinned, config.IntentTwinModeFeat); v != "" {
		t.Fatalf("expected feat mode to allow untwinned SUB fix commit, got %q", v)
	}
	if v := ValidateCommitIntentTwinMode(subFixUntwinned, config.IntentTwinModeStrict); v == "" {
		t.Fatal("expected strict mode to reject untwinned SUB fix commit")
	}
	if v := ValidateCommitIntentTwinMode(intDocsUntwinned, config.IntentTwinModeStrict); v != "" {
		t.Fatalf("expected strict mode to allow INT commit without twin, got %q", v)
	}
}

func TestThresholdIntegrityGate_RejectsIntentTwinModeDowngrade(t *testing.T) {
	trunkDir := t.TempDir()
	wtDir := t.TempDir()

	trunkYAML := "project: demo\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: true\n    intent_twin_mode: feat\n"
	wtYAML := "project: demo\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n  quality_gates:\n    strict_mode: true\n    intent_twin_mode: off\n"

	if err := os.WriteFile(filepath.Join(trunkDir, "nomos.yaml"), []byte(trunkYAML), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtDir, "nomos.yaml"), []byte(wtYAML), 0600); err != nil {
		t.Fatal(err)
	}

	gate := &ThresholdIntegrityGate{}
	res, err := gate.Evaluate(&vscope.VerifyContext{RepoRoot: trunkDir, WorktreePath: wtDir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Passed || !strings.Contains(res.Message, "intent_twin_mode") {
		t.Fatalf("expected ThresholdIntegrityGate to reject intent_twin_mode downgrade, got %+v", res)
	}
}
