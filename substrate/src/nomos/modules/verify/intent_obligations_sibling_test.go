package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
)

// @verifies TC-OBL-015
func TestComputePendingSiblingDeltasFromTasks_SharedSpecIsolation(t *testing.T) {
	repo := initTestGitRepoWithBaseSpec(t)
	commitSharedSpecMutation(t, repo, "[INT-10] feat(intent): add alpha obligation",
		"# Shared\n```go\npackage p\ntype Box struct {\n\tID string\n\tAlpha int\n}\n```\n| ID | Desc |\n|---|---|\n| `TC-SHR-01` | Base |\n| `TC-ALP-01` | Alpha |\n")
	commitSharedSpecMutation(t, repo, "[INT-20] feat(intent): add beta obligation",
		"# Shared\n```go\npackage p\ntype Box struct {\n\tID string\n\tAlpha int\n\tBeta bool\n}\n```\n| ID | Desc |\n|---|---|\n| `TC-SHR-01` | Base |\n| `TC-ALP-01` | Alpha |\n| `TC-BET-01` | Beta |\n")

	tasks := []task.TaskRecord{
		{Key: "SUB-10", Status: task.StatusEdit, TwinKey: "INT-10"},
		{Key: "SUB-20", Status: task.StatusEdit, TwinKey: "INT-20"},
	}
	delta := computePendingSiblingDeltasFromTasks(repo, repo, "SUB-10", tasks)
	if !delta.HasAddedTC("TC-BET-01") || !delta.HasAddedField("Box", "Beta") {
		t.Fatalf("expected sibling INT-20 additions (TC-BET-01, Box.Beta) to be deferred, got: %+v", delta)
	}
	if delta.HasAddedTC("TC-ALP-01") || delta.HasAddedField("Box", "Alpha") {
		t.Fatalf("expected own twin INT-10 additions (TC-ALP-01, Box.Alpha) NOT to be deferred, got: %+v", delta)
	}
}

func initTestGitRepoWithBaseSpec(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGitOrFail(t, repo, "init")
	runGitOrFail(t, repo, "config", "user.email", "test@example.com")
	runGitOrFail(t, repo, "config", "user.name", "Test")
	commitSharedSpecMutation(t, repo, "chore: base spec",
		"# Shared\n```go\npackage p\ntype Box struct {\n\tID string\n}\n```\n| ID | Desc |\n|---|---|\n| `TC-SHR-01` | Base |\n")
	return repo
}

func commitSharedSpecMutation(t *testing.T, repo, msg, body string) {
	t.Helper()
	specPath := filepath.Join(repo, "intent", "architecture", "specs", "shared_spec.md")
	if err := os.MkdirAll(filepath.Dir(specPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(specPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	runGitOrFail(t, repo, "add", ".")
	runGitOrFail(t, repo, "commit", "-m", msg)
}

func runGitOrFail(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := workspace.SafeGitCommand(dir, args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
	}
}
