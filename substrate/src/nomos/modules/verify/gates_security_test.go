package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func TestSecurityGatesRegistration(t *testing.T) {
	for _, id := range []string{"secret", "ip_leak", "broken_wire", "config_immutability"} {
		g := GateByID(id)
		if g == nil {
			t.Fatalf("expected gate %s to be registered", id)
		}
	}
}

func TestSecretGateClean(t *testing.T) {
	g := &SecretGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected secret gate to pass on clean workspace: %s", res.Message)
	}
}

func TestBrokenWireGateClean(t *testing.T) {
	g := &BrokenWireGate{}
	ctx := &VerifyContext{RepoRoot: "."}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected broken wire gate to pass on clean workspace: %s", res.Message)
	}
}

func TestConfigImmutabilityGate(t *testing.T) {
	g := &ConfigImmutabilityGate{}
	ctx := &VerifyContext{TaskID: "INT-737"}
	res, err := g.Evaluate(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected intent task to pass config immutability: %s", res.Message)
	}
}

func TestIsIntentConfigPath(t *testing.T) {
	tests := []struct {
		path           string
		intentRoots    []string
		substrateRoots []string
		want           bool
	}{
		{"nomos.yaml", nil, nil, true},
		{".nomos.yaml", nil, nil, false},
		{"nomos.yml", nil, nil, false},
		{"intent/config/nomos.yaml", nil, nil, true},
		{"substrate/cli/templates/nomos.yaml", nil, nil, false},
		{"src/templates/nomos.yaml", []string{"docs"}, []string{"src"}, false},
		{"docs/config/nomos.yaml", []string{"docs"}, []string{"src"}, true},
		{"substrate/main.go", nil, nil, false},
	}

	for _, tt := range tests {
		got := isIntentConfigPath(tt.path, tt.intentRoots, tt.substrateRoots)
		if got != tt.want {
			t.Errorf("isIntentConfigPath(%q) = %v; want %v", tt.path, got, tt.want)
		}
	}
}

func TestSecretGateGitIgnoredBuildDirsVsUnignoredDirs(t *testing.T) {
	dir := t.TempDir()
	if out, err := workspace.SafeGitCommand(dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v (%s)", err, string(out))
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("build/\n.svelte-kit/\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	syntheticKey := "AKIA" + "IOSFODNN7EXAMPLE"
	for _, rel := range []string{
		filepath.Join("app", "build", "client", "chunk.js"),
		filepath.Join("app", ".svelte-kit", "output", "chunk.js"),
	} {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(`const k = "`+syntheticKey+`";`), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	g := &SecretGate{}
	res, err := g.Evaluate(&VerifyContext{WorktreePath: dir})
	if err != nil {
		t.Fatalf("unexpected error on gitignored build dirs: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected gitignored build/.svelte-kit dirs to be skipped, got: %s", res.Message)
	}

	// Now create a non-ignored directory named "build" in a fresh repo without .gitignore.
	unignoredRepo := t.TempDir()
	if out, err := workspace.SafeGitCommand(unignoredRepo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init unignoredRepo failed: %v (%s)", err, string(out))
	}
	leakFile := filepath.Join(unignoredRepo, "build", "deploy.sh")
	if err := os.MkdirAll(filepath.Dir(leakFile), 0o755); err != nil {
		t.Fatalf("mkdir unignored build: %v", err)
	}
	if err := os.WriteFile(leakFile, []byte("export AWS_ACCESS_KEY_ID="+syntheticKey+"\n"), 0o644); err != nil {
		t.Fatalf("write unignored leak: %v", err)
	}

	res2, err := g.Evaluate(&VerifyContext{WorktreePath: unignoredRepo})
	if err != nil {
		t.Fatalf("unexpected error on unignored build dir: %v", err)
	}
	if res2.Passed {
		t.Fatalf("expected unignored build/deploy.sh secret leak to fail closed")
	}
}
