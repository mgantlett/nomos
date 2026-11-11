package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type strictGoReleaserV2Config struct {
	Version       int                            `yaml:"version"`
	ProjectName   string                         `yaml:"project_name"`
	Before        strictGoReleaserBefore         `yaml:"before"`
	Builds        []strictGoReleaserBuild        `yaml:"builds"`
	Archives      []strictGoReleaserArchive      `yaml:"archives"`
	Checksum      strictGoReleaserChecksum       `yaml:"checksum"`
	HomebrewCasks []strictGoReleaserHomebrewCask `yaml:"homebrew_casks"`
	AURs          []strictGoReleaserAUR          `yaml:"aurs"`
	Changelog     strictGoReleaserChangelog      `yaml:"changelog"`
}

type strictGoReleaserBefore struct {
	Hooks []string `yaml:"hooks"`
}

type strictGoReleaserBuild struct {
	ID      string   `yaml:"id"`
	Dir     string   `yaml:"dir"`
	Main    string   `yaml:"main"`
	Binary  string   `yaml:"binary"`
	Flags   []string `yaml:"flags"`
	Ldflags []string `yaml:"ldflags"`
	Env     []string `yaml:"env"`
	Goos    []string `yaml:"goos"`
	Goarch  []string `yaml:"goarch"`
}

type strictGoReleaserArchive struct {
	ID              string                           `yaml:"id"`
	Formats         []string                         `yaml:"formats"`
	NameTemplate    string                           `yaml:"name_template"`
	FormatOverrides []strictGoReleaserFormatOverride `yaml:"format_overrides"`
	Files           []string                         `yaml:"files"`
}

type strictGoReleaserFormatOverride struct {
	Goos    string   `yaml:"goos"`
	Formats []string `yaml:"formats"`
}

type strictGoReleaserChecksum struct {
	NameTemplate string `yaml:"name_template"`
}

// strictGoReleaserAUR mirrors GoReleaser v2 config.AUR fields so invalid keys (e.g. options) fail closed in unit tests.
type strictGoReleaserAUR struct {
	Name          string   `yaml:"name"`
	IDs           []string `yaml:"ids"`
	Homepage      string   `yaml:"homepage"`
	Description   string   `yaml:"description"`
	Maintainers   []string `yaml:"maintainers"`
	Contributors  []string `yaml:"contributors"`
	License       string   `yaml:"license"`
	PrivateKey    string   `yaml:"private_key"`
	GitURL        string   `yaml:"git_url"`
	GitSSHCommand string   `yaml:"git_ssh_command"`
	SkipUpload    string   `yaml:"skip_upload"`
	URLTemplate   string   `yaml:"url_template"`
	CommitMsgTmpl string   `yaml:"commit_msg_template"`
	Directory     string   `yaml:"directory"`
	Conflicts     []string `yaml:"conflicts"`
	Provides      []string `yaml:"provides"`
	Depends       []string `yaml:"depends"`
	OptDepends    []string `yaml:"optdepends"`
	Backup        []string `yaml:"backup"`
	Rel           string   `yaml:"rel"`
	Package       string   `yaml:"package"`
}

type strictGoReleaserChangelog struct {
	Sort    string                        `yaml:"sort"`
	Disable string                        `yaml:"disable"`
	Filters strictGoReleaserChangelogFilt `yaml:"filters"`
}

type strictGoReleaserChangelogFilt struct {
	Exclude []string `yaml:"exclude"`
	Include []string `yaml:"include"`
}

func decodeStrictGoReleaserV2(raw []byte) (*strictGoReleaserV2Config, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var cfg strictGoReleaserV2Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func TestGoReleaserV2StrictSchemaAndEnvSafety(t *testing.T) {
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
		t.Fatalf(".goreleaser.yaml failed strict GoReleaser v2 KnownFields schema check: %v", err)
	}
	if cfg.Version != 2 {
		t.Fatalf("expected GoReleaser version 2, got %d", cfg.Version)
	}
	if len(cfg.AURs) == 0 {
		t.Fatalf("expected at least 1 AUR target in .goreleaser.yaml")
	}
	for _, aur := range cfg.AURs {
		if strings.Contains(aur.PrivateKey, ".Env.") || strings.Contains(aur.SkipUpload, ".Env.") {
			t.Fatalf("AUR target %s uses unguarded .Env.<KEY> lookup (use index .Env \"KEY\" to avoid missingkey errors)", aur.Name)
		}
	}

	// Regression vector: injecting unsupported `options:` under `aurs:` MUST fail closed immediately.
	mutated := strings.Replace(string(raw), "    package: |-", "    options:\n      - \"!strip\"\n    package: |-", 1)
	if _, mErr := decodeStrictGoReleaserV2([]byte(mutated)); mErr == nil {
		t.Fatalf("expected unsupported options field in config.AUR to fail strict KnownFields check")
	}

	// Verify CI workflow executes goreleaser check on main/PRs before any release tag is cut.
	ciRaw, ciErr := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "ci.yaml"))
	if ciErr != nil {
		t.Fatalf("read ci.yaml: %v", ciErr)
	}
	if !strings.Contains(string(ciRaw), "goreleaser/goreleaser-action") || !strings.Contains(string(ciRaw), "args: check") {
		t.Fatalf("expected .github/workflows/ci.yaml to enforce goreleaser check pre-flight")
	}
}

func runRenderReleaseNotes(t *testing.T, repoRoot string, tag string) (string, error) {
	t.Helper()
	bashBin, err := exec.LookPath("bash")
	if err != nil {
		return "", err
	}
	scriptPath := filepath.Join(repoRoot, "packaging", "render-release-notes.sh")
	cmd := exec.Command(bashBin, scriptPath, tag)
	cmd.Dir = repoRoot
	out, runErr := cmd.CombinedOutput()
	return string(out), runErr
}

func TestReleaseWorkflowPatchNotesRollup(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	relYaml, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "release.yaml"))
	if err != nil {
		t.Fatalf("read release.yaml: %v", err)
	}
	if !strings.Contains(string(relYaml), "packaging/render-release-notes.sh") {
		t.Fatalf("expected release.yaml to invoke canonical packaging/render-release-notes.sh")
	}

	patchNotes, pErr := runRenderReleaseNotes(t, repoRoot, "v4.10.1")
	if pErr != nil {
		t.Fatalf("render v4.10.1 release notes failed: %v (%s)", pErr, patchNotes)
	}
	for _, want := range []string{
		"## Changelog",
		"[SUB-1858]",
		"**Full Changelog**: https://github.com/mgantlett/nomos/compare/v4.10.0...v4.10.1",
		"<summary><strong>Release Notes &amp; Architecture Details</strong></summary>",
		"GoReleaser v2 `config.AUR` Schema Parity",
		"<summary><strong>Included <code>v4.10.0</code> Milestone Features</strong></summary>",
		".opencode/plugins/nomos.ts",
	} {
		if !strings.Contains(patchNotes, want) {
			t.Errorf("expected v4.10.1 notes to contain %q, got:\n%s", want, patchNotes)
		}
	}
	assertOrderedSubstrings(t, patchNotes, "> GoReleaser v2 AUR Schema Alignment", "## Changelog", "<details>", "Included <code>v4.10.0</code> Milestone Features")
}

func assertOrderedSubstrings(t *testing.T, doc string, tokens ...string) {
	t.Helper()
	cursor := -1
	for _, tok := range tokens {
		idx := strings.Index(doc, tok)
		if idx <= cursor {
			t.Fatalf("expected token %q after byte %d in rendered notes:\n%s", tok, cursor, doc)
		}
		cursor = idx
	}
}

func TestReleaseNotesHybridLedgerAndFailClosed(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	minorNotes, mErr := runRenderReleaseNotes(t, repoRoot, "v4.12.0")
	if mErr != nil {
		t.Fatalf("render v4.12.0 release notes failed: %v (%s)", mErr, minorNotes)
	}
	assertOrderedSubstrings(t, minorNotes, "> Zero-Config Brownfield Auto-Detection", "## Changelog", "[INT-1913]", "[SUB-1914]", "<details>", "Zero-Config Brownfield Onboarding")
	if !strings.Contains(minorNotes, "compare/v4.11.0...v4.12.0") {
		t.Errorf("expected compare link v4.11.0...v4.12.0 in v4.12.0 notes")
	}

	v413Notes, vErr := runRenderReleaseNotes(t, repoRoot, "v4.13.0")
	if vErr != nil {
		t.Fatalf("render v4.13.0 release notes failed: %v (%s)", vErr, v413Notes)
	}
	assertOrderedSubstrings(t, v413Notes, "> 11-Language Polyglot Architectural Obligation Proofs", "## Changelog", "compare/v4.12.1...v4.13.0", "<details>", "architecture_boundaries")

	v4131Notes, p1Err := runRenderReleaseNotes(t, repoRoot, "v4.13.1")
	if p1Err != nil {
		t.Fatalf("render v4.13.1 release notes failed: %v (%s)", p1Err, v4131Notes)
	}
	assertOrderedSubstrings(t, v4131Notes, "> Zero-Ceremony Multi-Agent Onboarding Invisibility", "## Changelog", "[SUB-2133]", "compare/v4.13.0...v4.13.1", "<details>", "Included <code>v4.13.0</code> Milestone Features")

	v414Notes, v414Err := runRenderReleaseNotes(t, repoRoot, "v4.14.0")
	if v414Err != nil {
		t.Fatalf("render v4.14.0 release notes failed: %v (%s)", v414Err, v414Notes)
	}
	assertOrderedSubstrings(t, v414Notes, "> 5-Provider Ambient Lifecycle Parity", "## Changelog", "[SUB-2188]", "compare/v4.13.2...v4.14.0", "<details>", "brew install --cask mgantlett/tap/nomos")

	genesisNotes, gErr := runRenderReleaseNotes(t, repoRoot, "v4.0.0")
	if gErr != nil {
		t.Fatalf("render v4.0.0 release notes failed: %v (%s)", gErr, genesisNotes)
	}
	if !strings.Contains(genesisNotes, "## Changelog") || strings.Contains(genesisNotes, "compare/") {
		t.Errorf("expected genesis v4.0.0 notes to include Changelog without compare link, got:\n%s", genesisNotes)
	}

	if _, badErr := runRenderReleaseNotes(t, repoRoot, "v99.99.99"); badErr == nil {
		t.Fatalf("expected missing SemVer tag v99.99.99 to fail closed")
	}
}
