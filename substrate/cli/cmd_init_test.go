package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

var (
	compiledHookTestOnce   sync.Once
	compiledHookTestBinDir string
	compiledHookTestErr    error
	initialCliTestDir      = resolveInitialCliTestDir()
)

func resolveInitialCliTestDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

func ensureCompiledHookTestEnv(t *testing.T) []string {
	t.Helper()
	compiledHookTestOnce.Do(func() {
		binDir, err := os.MkdirTemp("", "nomos-hook-test-bin-*")
		if err != nil {
			compiledHookTestErr = err
			return
		}
		subRoot := filepath.Dir(initialCliTestDir)
		buildCmd := exec.Command("go", "build", "-o", filepath.Join(binDir, "nomos"), "./cmd/nomos")
		buildCmd.Dir = subRoot
		if out, bErr := buildCmd.CombinedOutput(); bErr != nil {
			compiledHookTestErr = fmt.Errorf("%w: %s", bErr, string(out))
			return
		}
		compiledHookTestBinDir = binDir
	})
	if compiledHookTestErr != nil {
		t.Fatalf("failed building compiled nomos binary for hook test: %v", compiledHookTestErr)
	}
	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, "PATH=") {
			out = append(out, kv)
		}
	}
	return append(out, "PATH="+compiledHookTestBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func useCompiledNomosInPath(t *testing.T) {
	t.Helper()
	_ = ensureCompiledHookTestEnv(t)
	t.Setenv("PATH", compiledHookTestBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestExecuteInit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nomos-init-test-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer func() {
		if rErr := os.RemoveAll(tempDir); rErr != nil {
			// cleanup error in temp dir
		}
	}()

	gitDir := filepath.Join(tempDir, ".git")
	if mErr := os.MkdirAll(gitDir, 0755); mErr != nil {
		t.Fatalf("failed creating .git: %v", mErr)
	}

	if err := executeInit(tempDir); err != nil {
		t.Fatalf("executeInit failed: %v", err)
	}

	assertScaffoldedFiles(t, tempDir)
	assertScaffoldedHooks(t, tempDir)
}

func TestExecuteInit_NonGitDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nomos-init-nongit-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	if err := executeInit(tempDir); err != nil {
		t.Fatalf("executeInit on non-git dir failed: %v", err)
	}

	assertScaffoldedFiles(t, tempDir)
	assertScaffoldedHooks(t, tempDir)
}

func TestExecuteInit_MasterBranch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nomos-init-master-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	gitInitCmd := workspace.SafeGitCommand(tempDir, "init", "-b", "master")
	if out, err := gitInitCmd.CombinedOutput(); err != nil {
		// Fallback for older git versions
		initCmd := workspace.SafeGitCommand(tempDir, "init")
		if iOut, iErr := initCmd.CombinedOutput(); iErr != nil {
			t.Fatalf("git init failed: %v, out: %s / %s", iErr, out, iOut)
		}
		branchCmd := workspace.SafeGitCommand(tempDir, "checkout", "-b", "master")
		_ = branchCmd.Run()
	}

	if err := executeInit(tempDir); err != nil {
		t.Fatalf("executeInit on master branch repo failed: %v", err)
	}

	nomosYAML, err := os.ReadFile(filepath.Join(tempDir, "nomos.yaml"))
	if err != nil {
		t.Fatalf("failed reading nomos.yaml: %v", err)
	}
	if !strings.Contains(string(nomosYAML), "default_branch: \"master\"") {
		t.Errorf("expected default_branch: \"master\", got:\n%s", string(nomosYAML))
	}
}

func assertScaffoldedFiles(t *testing.T, dir string) {
	t.Helper()
	gitIgnorePath := filepath.Join(dir, ".gitignore")
	data, err := os.ReadFile(gitIgnorePath)
	if err != nil {
		t.Fatalf("failed reading .gitignore: %v", err)
	}
	if !strings.Contains(string(data), "worktrees/") || !strings.Contains(string(data), ".tmp/") {
		t.Errorf("expected boundaries in .gitignore, got: %s", string(data))
	}
	if !strings.Contains(string(data), "go.work") {
		t.Errorf("expected go.work in .gitignore, got: %s", string(data))
	}

	agentsData, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("failed reading AGENTS.md: %v", err)
	}
	if !strings.Contains(string(agentsData), "THE ONE-PAGE INVARIANT CARD") {
		t.Errorf("expected 7 laws in AGENTS.md, got: %s", string(agentsData))
	}

	claudeData, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed reading CLAUDE.md: %v", err)
	}
	if string(claudeData) != string(agentsData) {
		t.Errorf("expected byte parity between AGENTS.md and CLAUDE.md")
	}

	nomosYAML, err := os.ReadFile(filepath.Join(dir, "nomos.yaml"))
	if err != nil {
		t.Fatalf("expected nomos.yaml to exist: %v", err)
	}
	if !strings.Contains(string(nomosYAML), "default_branch:") {
		t.Errorf("expected default_branch in nomos.yaml, got: %s", string(nomosYAML))
	}
}

func assertScaffoldedHooks(t *testing.T, dir string) {
	t.Helper()
	agentsDir := filepath.Join(dir, ".agents")
	if _, err := os.Stat(filepath.Join(agentsDir, "workflows")); !os.IsNotExist(err) {
		t.Errorf("expected no legacy workflows directory to be scaffolded")
	}

	hooksManifest := filepath.Join(agentsDir, "hooks.json")
	manifestBytes, err := os.ReadFile(hooksManifest)
	if err != nil {
		t.Fatalf("expected hooks.json to exist: %v", err)
	}
	if !strings.Contains(string(manifestBytes), "session-handshake") {
		t.Errorf("expected session-handshake hook in hooks.json, got: %s", string(manifestBytes))
	}

	for _, sc := range []string{"session-start.sh", "worktree-guard.sh", "nomos-build-guard.sh", "post-edit-radar.sh"} {
		p := filepath.Join(agentsDir, "scripts", sc)
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatalf("expected hook script %s to exist: %v", sc, err)
		}
		if fi.Size() == 0 {
			t.Errorf("hook script %s is empty", sc)
		}
		if fi.Mode().Perm()&0111 == 0 {
			t.Errorf("expected hook script %s to be executable, mode=%v", sc, fi.Mode())
		}
		body, rErr := os.ReadFile(p)
		if rErr != nil {
			t.Fatalf("read %s: %v", sc, rErr)
		}
		if strings.Contains(string(body), "jq") || !strings.Contains(string(body), "exec ") {
			t.Errorf("expected %s to be a pure zero-jq exec launcher, got:\n%s", sc, string(body))
		}
	}
}

func TestExecuteInit_PrunesLegacyWorkflows(t *testing.T) {
	tempDir := t.TempDir()
	wfDir := filepath.Join(tempDir, ".agents", "workflows")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatalf("failed creating legacy workflows dir: %v", err)
	}
	for _, wf := range []string{"nomos-handshake.md", "nomos-idle.md", "nomos-review.md"} {
		if err := os.WriteFile(filepath.Join(wfDir, wf), []byte("# Legacy"), 0644); err != nil {
			t.Fatalf("failed writing legacy workflow: %v", err)
		}
	}

	if err := executeInit(tempDir); err != nil {
		t.Fatalf("executeInit failed: %v", err)
	}

	if _, err := os.Stat(wfDir); !os.IsNotExist(err) {
		t.Errorf("expected legacy workflows directory to be pruned, stat err=%v", err)
	}
	assertScaffoldedHooks(t, tempDir)
}

func TestMergeProtocolContent(t *testing.T) {
	tmpl := []byte("<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->\n# Nomos Protocol v4.0.0\n<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->")

	// Case 1: Empty input
	merged := mergeProtocolContent([]byte(""), tmpl)
	if !strings.Contains(string(merged), "# Nomos Protocol v4.0.0") {
		t.Errorf("expected template in empty merge: %s", string(merged))
	}

	// Case 2: Brownfield without markers (preserves custom instructions)
	existingCustom := []byte("# Custom Project Rules\n- Use FastAPI\n- SQLite DB")
	mergedBrownfield := mergeProtocolContent(existingCustom, tmpl)
	mStr := string(mergedBrownfield)
	if !strings.Contains(mStr, "# Nomos Protocol v4.0.0") {
		t.Errorf("expected Nomos protocol in brownfield merge")
	}
	if !strings.Contains(mStr, "# Custom Project Rules") || !strings.Contains(mStr, "- Use FastAPI") {
		t.Errorf("expected custom rules preserved in brownfield merge: %s", mStr)
	}

	// Case 3: In-place update when markers already exist
	existingWithMarkers := []byte("# Header\n<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->\n# Outdated v3.9.0\n<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->\n# Footer")
	updated := mergeProtocolContent(existingWithMarkers, tmpl)
	uStr := string(updated)
	if !strings.Contains(uStr, "# Header") || !strings.Contains(uStr, "# Footer") {
		t.Errorf("expected header and footer preserved: %s", uStr)
	}
	if !strings.Contains(uStr, "# Nomos Protocol v4.0.0") {
		t.Errorf("expected updated protocol in markers: %s", uStr)
	}
	if strings.Contains(uStr, "# Outdated v3.9.0") {
		t.Errorf("expected outdated protocol replaced: %s", uStr)
	}
}

func TestExecuteInit_BrownfieldMerge(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nomos-init-brownfield-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// Pre-create brownfield AGENTS.md and CLAUDE.md with custom team instructions
	customAgents := "# Team Alpha Guidelines\n- Always use PostgreSQL\n- Run npm test before commit\n"
	if err := os.WriteFile(filepath.Join(tempDir, "AGENTS.md"), []byte(customAgents), 0644); err != nil {
		t.Fatalf("failed writing brownfield AGENTS.md: %v", err)
	}

	customClaude := "# Claude Rules\n- Output concise markdown\n"
	if err := os.WriteFile(filepath.Join(tempDir, "CLAUDE.md"), []byte(customClaude), 0644); err != nil {
		t.Fatalf("failed writing brownfield CLAUDE.md: %v", err)
	}

	if err := executeInit(tempDir); err != nil {
		t.Fatalf("executeInit failed on brownfield: %v", err)
	}

	// Verify AGENTS.md retains custom guidelines AND includes Nomos protocol
	agentsData, err := os.ReadFile(filepath.Join(tempDir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("failed reading AGENTS.md: %v", err)
	}
	aStr := string(agentsData)
	if !strings.Contains(aStr, "Team Alpha Guidelines") || !strings.Contains(aStr, "PostgreSQL") {
		t.Errorf("custom guidelines were lost in brownfield AGENTS.md merge:\n%s", aStr)
	}
	if !strings.Contains(aStr, "THE ONE-PAGE INVARIANT CARD") {
		t.Errorf("Nomos invariant card was not added to brownfield AGENTS.md:\n%s", aStr)
	}

	// Verify CLAUDE.md retains custom instructions AND includes Nomos protocol
	claudeData, err := os.ReadFile(filepath.Join(tempDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed reading CLAUDE.md: %v", err)
	}
	cStr := string(claudeData)
	if !strings.Contains(cStr, "Claude Rules") || !strings.Contains(cStr, "concise markdown") {
		t.Errorf("custom rules were lost in brownfield CLAUDE.md merge:\n%s", cStr)
	}
	if !strings.Contains(cStr, "THE ONE-PAGE INVARIANT CARD") {
		t.Errorf("Nomos invariant card was not added to brownfield CLAUDE.md:\n%s", cStr)
	}
}

func TestExecuteInit_BrownfieldGitRepoAndVerify(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nomos-brownfield-git-*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	initGitBrownfieldRepo(t, tempDir)

	if err := executeInit(tempDir); err != nil {
		t.Fatalf("executeInit failed on brownfield git repo: %v", err)
	}

	if err := runVerification(tempDir, "", true, true, false); err != nil {
		t.Fatalf("expected brownfield repo to pass all 42 verification gates immediately after nomos init, got: %v", err)
	}
}

func initGitBrownfieldRepo(t *testing.T, dir string) {
	t.Helper()
	_ = workspace.SafeGitCommand(dir, "init", "-b", "main").Run()
	_ = workspace.SafeGitCommand(dir, "config", "user.email", "dev@nomos.local").Run()
	_ = workspace.SafeGitCommand(dir, "config", "user.name", "Cleanroom Agent").Run()

	srcDir := filepath.Join(dir, "substrate", "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("failed creating src dir: %v", err)
	}
	pyCode := "\"\"\"Brownfield Python service.\"\"\"\n\ndef compute_score(val: int) -> int:\n    \"\"\"Compute score.\"\"\"\n    if val > 0:\n        return val * 2\n    return 0\n"
	if err := os.WriteFile(filepath.Join(srcDir, "service.py"), []byte(pyCode), 0644); err != nil {
		t.Fatalf("failed writing service.py: %v", err)
	}
	_ = workspace.SafeGitCommand(dir, "add", ".").Run()
	_ = workspace.SafeGitCommand(dir, "commit", "-m", "chore: initial brownfield commit").Run()
}

