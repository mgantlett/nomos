package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

func initSurfaceTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runSurfaceGit(t, repo, "init", "-b", "main")
	runSurfaceGit(t, repo, "config", "user.name", "Nomos Test")
	runSurfaceGit(t, repo, "config", "user.email", "test@sophialabs.ca")

	cliFile := filepath.Join(repo, "substrate", "cli", "cmd_demo.go")
	if err := os.MkdirAll(filepath.Dir(cliFile), 0755); err != nil {
		t.Fatal(err)
	}
	baseSrc := `package cli

import "github.com/spf13/cobra"

type DemoConfig struct {
	Model string ` + "`yaml:\"model\"`" + `
}

func NewDemoCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{Use: "demo"}
	cmd.Flags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	return cmd
}
`
	if err := os.WriteFile(cliFile, []byte(baseSrc), 0644); err != nil {
		t.Fatal(err)
	}
	tplFile := filepath.Join(repo, "substrate", "cli", "templates", "hooks", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(tplFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tplFile, []byte(`{"version":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	runSurfaceGit(t, repo, "add", "-A")
	runSurfaceGit(t, repo, "commit", "-m", "[INT-42] docs(specs): seed demo baseline\n\nSeed baseline spec and command.\n\n___\n\nTask: INT-42")
	return repo
}

func runSurfaceGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	out, err := workspace.SafeGitCommand(dir, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

// @verifies TC-CMT-006 TC-CMT-007 TC-CMT-008
func TestValidateWorktreeIntentTwin_SurfaceDeltaAndProvenance(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	repo := initSurfaceTestRepo(t)
	runSurfaceGit(t, repo, "checkout", "-b", "feature/SUB-900")

	cliFile := filepath.Join(repo, "substrate", "cli", "cmd_demo.go")

	// 1. TC-CMT-007: Internal helper refactor with zero CLI/YAML/template surface delta passes untwinned under feat mode.
	internalRefactorSrc := `package cli

import "github.com/spf13/cobra"

type DemoConfig struct {
	Model string ` + "`yaml:\"model\"`" + `
}

func normalizeLabel(s string) string { return s }

func NewDemoCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{Use: "demo", Short: normalizeLabel("run demo")}
	cmd.Flags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	return cmd
}
`
	if err := os.WriteFile(cliFile, []byte(internalRefactorSrc), 0644); err != nil {
		t.Fatal(err)
	}
	refactorUntwinned := "[SUB-900] refactor(cli): extract internal label normalizer\n\nExtract private helper without changing CLI flags or YAML config contracts.\n\n___\n\nTask: SUB-900"
	if v := ValidateWorktreeIntentTwin(repo, refactorUntwinned, config.IntentTwinModeFeat); v != "" {
		t.Fatalf("TC-CMT-007: expected internal refactor with empty NormativeSurfaceDelta to pass untwinned, got: %s", v)
	}

	// 2. TC-CMT-006: Refactor adding a new CLI flag and YAML field fails closed when untwinned.
	surfaceExpandSrc := `package cli

import "github.com/spf13/cobra"

type DemoConfig struct {
	Model          string ` + "`yaml:\"model\"`" + `
	ThinkingBudget int    ` + "`yaml:\"thinking_budget,omitempty\"`" + `
}

func NewDemoCmd() *cobra.Command {
	var verbose, antigravity bool
	cmd := &cobra.Command{Use: "demo"}
	cmd.Flags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	cmd.Flags().BoolVar(&antigravity, "antigravity", false, "enable Antigravity 2.0 hook lifecycle")
	return cmd
}
`
	if err := os.WriteFile(cliFile, []byte(surfaceExpandSrc), 0644); err != nil {
		t.Fatal(err)
	}
	delta := InspectWorktreeNormativeSurface(repo)
	if !delta.HasChanges() || len(delta.CLISurface) == 0 || len(delta.ConfigSchema) == 0 {
		t.Fatalf("TC-CMT-006: expected CLI and YAML surface delta, got: %+v", delta)
	}
	vUntwinned := ValidateWorktreeIntentTwin(repo, refactorUntwinned, config.IntentTwinModeFeat)
	if !strings.Contains(vUntwinned, "modifies normative contract surface") || !strings.Contains(vUntwinned, "--antigravity") || !strings.Contains(vUntwinned, "DemoConfig.thinking_budget") {
		t.Fatalf("TC-CMT-006: expected normative surface violation naming --antigravity and DemoConfig.thinking_budget, got: %s", vUntwinned)
	}

	// 3. TC-CMT-006 pass when bound to real trunk commit [INT-42].
	refactorTwinnedGit := refactorUntwinned + "\nTwin: INT-42"
	if v := ValidateWorktreeIntentTwin(repo, refactorTwinnedGit, config.IntentTwinModeFeat); v != "" {
		t.Fatalf("TC-CMT-006: expected surface-expanding refactor twinned with trunk INT-42 to pass, got: %s", v)
	}

	// 4. TC-CMT-008: Unresolvable Twin: INT-999999 rejected when absent from Git history and Living Ledger.
	refactorBogusTwin := refactorUntwinned + "\nTwin: INT-999999"
	vBogus := ValidateWorktreeIntentTwin(repo, refactorBogusTwin, config.IntentTwinModeFeat)
	if !strings.Contains(vBogus, "unresolvable 'Twin: INT-999999'") {
		t.Fatalf("TC-CMT-008: expected unresolvable Twin error for INT-999999, got: %s", vBogus)
	}

	// 5. TC-CMT-008 pass when Twin: INT-55 is registered in the SQLite Living Ledger.
	db, err := storage.OpenDB(config.ResolveGlobalDBPath())
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			t.Errorf("Close: %v", cErr)
		}
	}()
	store := task.NewSQLiteTaskStore(db)
	if err := store.CreateTask(task.TaskRecord{
		Key:         "INT-55",
		Title:       "Author normative hook specification",
		Description: "## Scope & Context\nSpec update targeting intent/architecture/specs/nomos_hermetic_worktree_spec.md.\n\n## Acceptance Criteria\n- [ ] Update spec\n\n## Verification\n- nomos verify",
		Plane:       "intent",
		SchemaType:  task.SchemaIntent,
		Tier:        task.Tier1,
		Status:      task.StatusInProgress,
		Priority:    50,
		Burden:      1,
	}); err != nil {
		t.Fatalf("CreateTask INT-55: %v", err)
	}
	refactorLedgerTwin := refactorUntwinned + "\nTwin: INT-55"
	if v := ValidateWorktreeIntentTwin(repo, refactorLedgerTwin, config.IntentTwinModeFeat); v != "" {
		t.Fatalf("TC-CMT-008: expected Twin: INT-55 backed by Living Ledger to pass, got: %s", v)
	}
}
