// Package cli verifies GoReleaser v2 Homebrew Cask, quarantine, and substrate category contracts.
// @intent intent.md
package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// strictGoReleaserHomebrewCask mirrors GoReleaser v2.20+ config.HomebrewCask fields while omitting
// deprecated keys (singular binary, hooks.post.install, url.verified) so KnownFields(true) fails closed.
type strictGoReleaserHomebrewCask struct {
	Name          string                    `yaml:"name"`
	IDs           []string                  `yaml:"ids"`
	Binaries      []string                  `yaml:"binaries"`
	Manpages      []string                  `yaml:"manpages"`
	Homepage      string                    `yaml:"homepage"`
	Description   string                    `yaml:"description"`
	License       string                    `yaml:"license"`
	Directory     string                    `yaml:"directory"`
	SkipUpload    string                    `yaml:"skip_upload"`
	URL           strictGoReleaserCaskURL   `yaml:"url"`
	CommitMsgTmpl string                    `yaml:"commit_msg_template"`
	Repository    strictGoReleaserCaskRepo  `yaml:"repository"`
	Hooks         strictGoReleaserCaskHooks `yaml:"hooks"`
	Caveats       string                    `yaml:"caveats"`
}

type strictGoReleaserCaskURL struct {
	Template string `yaml:"template"`
}

type strictGoReleaserCaskRepo struct {
	Owner  string `yaml:"owner"`
	Name   string `yaml:"name"`
	Branch string `yaml:"branch"`
	Token  string `yaml:"token"`
}

type strictGoReleaserCaskHooks struct {
	Pre  strictGoReleaserCaskHookBlock `yaml:"pre"`
	Post strictGoReleaserCaskHookBlock `yaml:"post"`
}

type strictGoReleaserCaskHookBlock struct {
	InstallSteps   string `yaml:"install_steps"`
	UninstallSteps string `yaml:"uninstall_steps"`
}

func TestGoReleaserHomebrewCaskStrictSchemaAndEnvSafety(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(repoRoot, ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read .goreleaser.yaml: %v", err)
	}
	cfg, err := decodeStrictGoReleaserV2(raw)
	if err != nil {
		t.Fatalf("decode .goreleaser.yaml: %v", err)
	}
	assertCanonicalNomosCask(t, cfg.HomebrewCasks)
	assertDeprecatedCaskMutationsFailClosed(t, string(raw))
}

func assertCanonicalNomosCask(t *testing.T, casks []strictGoReleaserHomebrewCask) {
	t.Helper()
	if len(casks) == 0 {
		t.Fatalf("expected at least 1 homebrew_casks target in .goreleaser.yaml")
	}
	cask := casks[0]
	if cask.Name != "nomos" || !slices.Contains(cask.Binaries, "nomos") {
		t.Fatalf("expected homebrew_casks[0] name=nomos with binaries=[nomos], got %+v", cask)
	}
	if cask.Repository.Owner != "mgantlett" || cask.Repository.Name != "homebrew-tap" || cask.Repository.Branch != "main" {
		t.Fatalf("expected mgantlett/homebrew-tap@main repository target, got %+v", cask.Repository)
	}
	const wantEnvGuard = `index .Env "HOMEBREW_TAP_GITHUB_TOKEN"`
	if !strings.Contains(cask.Repository.Token, wantEnvGuard) || !strings.Contains(cask.SkipUpload, wantEnvGuard) {
		t.Fatalf("expected guarded %s in token and skip_upload, got token=%q skip_upload=%q", wantEnvGuard, cask.Repository.Token, cask.SkipUpload)
	}
	if strings.Contains(cask.Repository.Token, ".Env.") || strings.Contains(cask.SkipUpload, ".Env.") {
		t.Fatalf("homebrew_casks target %s uses unguarded .Env.<KEY> lookup", cask.Name)
	}
	if !strings.Contains(cask.Hooks.Post.InstallSteps, "on_macos do") || !strings.Contains(cask.Hooks.Post.InstallSteps, "com.apple.quarantine") {
		t.Fatalf("expected macOS Gatekeeper quarantine clearing in hooks.post.install_steps, got %q", cask.Hooks.Post.InstallSteps)
	}
}

func assertDeprecatedCaskMutationsFailClosed(t *testing.T, baseYAML string) {
	t.Helper()
	cases := []struct {
		name    string
		oldFrag string
		newFrag string
	}{
		{
			name:    "deprecated hooks.post.install fails KnownFields(true)",
			oldFrag: "install_steps: |-",
			newFrag: "install: |-",
		},
		{
			name:    "deprecated singular binary key fails KnownFields(true)",
			oldFrag: "    binaries:\n      - nomos",
			newFrag: "    binary: nomos",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(baseYAML, tc.oldFrag) {
				t.Fatalf("base .goreleaser.yaml missing expected fragment %q", tc.oldFrag)
			}
			mutated := strings.Replace(baseYAML, tc.oldFrag, tc.newFrag, 1)
			if _, err := decodeStrictGoReleaserV2([]byte(mutated)); err == nil {
				t.Fatalf("expected %s to fail decodeStrictGoReleaserV2 KnownFields(true)", tc.name)
			}
		})
	}
}

func TestReleaseWorkflowInstallerAndSubstrateCategoryParity(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	cases := []repoContractCase{
		{
			name:     "release workflow passes HOMEBREW_TAP_GITHUB_TOKEN to GoReleaser",
			relPath:  filepath.Join(".github", "workflows", "release.yaml"),
			mustHave: []string{"HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}"},
		},
		{
			name:     "install.sh clears com.apple.quarantine on Darwin safely",
			relPath:  "install.sh",
			mustHave: []string{`[ "$OS" = "Darwin" ]`, `xattr -dr com.apple.quarantine "$INSTALL_DIR/nomos"`},
		},
		{
			name:     "nomos.yaml scaffold template frames Nomos execution substrate",
			relPath:  filepath.Join("substrate", "cli", "templates", "nomos.yaml"),
			mustHave: []string{"Nomos execution substrate"},
			forbid:   []string{"Nomos control harness"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, readErr := os.ReadFile(filepath.Join(repoRoot, tc.relPath))
			if readErr != nil {
				t.Fatalf("read %s: %v", tc.relPath, readErr)
			}
			assertContractSubstrings(t, tc.relPath, string(raw), tc.mustHave, tc.forbid)
		})
	}

	rootCmd := NewRootCmd()
	if rootCmd.Short != "Deterministic execution substrate between coding agents and Git" {
		t.Fatalf("unexpected rootCmd.Short: %q", rootCmd.Short)
	}
	if !strings.HasPrefix(rootCmd.Long, "Nomos Open-Core Deterministic Execution Substrate\n") {
		t.Fatalf("unexpected rootCmd.Long header: %q", rootCmd.Long)
	}
}

func assertContractSubstrings(t *testing.T, relPath, body string, mustHave, forbid []string) {
	t.Helper()
	for _, want := range mustHave {
		if !strings.Contains(body, want) {
			t.Errorf("%s missing required substring %q", relPath, want)
		}
	}
	for _, bad := range forbid {
		if strings.Contains(body, bad) {
			t.Errorf("%s contains prohibited substring %q", relPath, bad)
		}
	}
}
