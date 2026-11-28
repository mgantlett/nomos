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
	DockersV2     []strictGoReleaserDockerV2     `yaml:"dockers_v2"`
	HomebrewCasks []strictGoReleaserHomebrewCask `yaml:"homebrew_casks"`
	Scoops        []strictGoReleaserScoop        `yaml:"scoops"`
	Winget        []strictGoReleaserWinget       `yaml:"winget"`
	AURs          []strictGoReleaserAUR          `yaml:"aurs"`
	NFPMs         []strictGoReleaserNFPM         `yaml:"nfpms"`
	Changelog     strictGoReleaserChangelog      `yaml:"changelog"`
}

type strictGoReleaserNFPMContent struct {
	Src string `yaml:"src"`
	Dst string `yaml:"dst"`
}

type strictGoReleaserNFPM struct {
	ID          string                        `yaml:"id"`
	PackageName string                        `yaml:"package_name"`
	IDs         []string                      `yaml:"ids"`
	Homepage    string                        `yaml:"homepage"`
	Maintainer  string                        `yaml:"maintainer"`
	Description string                        `yaml:"description"`
	License     string                        `yaml:"license"`
	Formats     []string                      `yaml:"formats"`
	Bindir      string                        `yaml:"bindir"`
	Contents    []strictGoReleaserNFPMContent `yaml:"contents"`
}

type strictGoReleaserScoopRepo struct {
	Owner, Name, Branch, Token string
}

type strictGoReleaserScoop struct {
	Name, Directory, Homepage, Description, License string
	SkipUpload                                      string `yaml:"skip_upload"`
	Repository                                      strictGoReleaserScoopRepo
}

type strictGoReleaserWinget struct {
	Name, Publisher, Description, Homepage, License string
	PackageIdentifier                               string `yaml:"package_identifier"`
	ShortDescription                                string `yaml:"short_description"`
	LicenseURL                                      string `yaml:"license_url"`
	ReleaseNotesURL                                 string `yaml:"release_notes_url"`
	Tags                                            []string
	SkipUpload                                      string `yaml:"skip_upload"`
	Repository                                      strictGoReleaserScoopRepo
}

type strictGoReleaserDockerV2 struct {
	ID         string            `yaml:"id"`
	Dockerfile string            `yaml:"dockerfile"`
	IDs        []string          `yaml:"ids"`
	Images     []string          `yaml:"images"`
	Tags       []string          `yaml:"tags"`
	Platforms  []string          `yaml:"platforms"`
	ExtraFiles []string          `yaml:"extra_files"`
	Labels     map[string]string `yaml:"labels"`
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
		if strings.TrimSpace(aur.PrivateKey) != "{{ .Env.AUR_KEY }}" {
			t.Fatalf("AUR target %s private_key must use GoReleaser v2.20+ {{ .Env.AUR_KEY }}, got %q", aur.Name, aur.PrivateKey)
		}
		if !strings.Contains(aur.SkipUpload, `index .Env "AUR_KEY"`) || strings.Contains(aur.SkipUpload, ".Env.") {
			t.Fatalf("AUR target %s skip_upload uses unguarded .Env.<KEY> lookup (use index .Env \"KEY\" to avoid missingkey errors)", aur.Name)
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
	ciYaml, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "ci.yaml"))
	if err != nil {
		t.Fatalf("read ci.yaml: %v", err)
	}
	if !strings.Contains(string(ciYaml), "packaging/render-release-notes.sh") || !strings.Contains(string(ciYaml), "needs: verify") {
		t.Fatalf("expected unified ci.yaml to gate GoReleaser on verify and invoke canonical packaging/render-release-notes.sh")
	}
	if _, dupErr := os.Stat(filepath.Join(repoRoot, ".github", "workflows", "release.yaml")); !os.IsNotExist(dupErr) {
		t.Fatalf("expected duplicate .github/workflows/release.yaml to be removed so release pushes run a single workflow")
	}

	patchNotes, pErr := runRenderReleaseNotes(t, repoRoot, "v4.10.1")
	if pErr != nil {
		t.Fatalf("render v4.10.1 release notes failed: %v (%s)", pErr, patchNotes)
	}
	for _, want := range []string{
		"## Changelog",
		"[SUB-1858]",
		"**Full Changelog**: https://github.com/mgantlett/nomos/compare/v4.10.0...v4.10.1",
		"<summary><strong>Verified 2PC Commit Ledger</strong></summary>",
		"GoReleaser v2 `config.AUR` Schema Parity",
		"<summary><strong>Included <code>v4.10.0</code> Milestone Features</strong></summary>",
		".opencode/plugins/nomos.ts",
	} {
		if !strings.Contains(patchNotes, want) {
			t.Errorf("expected v4.10.1 notes to contain %q, got:\n%s", want, patchNotes)
		}
	}
	assertOrderedSubstrings(t, patchNotes, "> GoReleaser v2 AUR Schema Alignment", "GoReleaser v2 `config.AUR` Schema Parity", "<details>", "## Changelog", "Included <code>v4.10.0</code> Milestone Features")
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
	assertOrderedSubstrings(t, minorNotes, "> Zero-Config Brownfield Auto-Detection", "Zero-Config Brownfield Onboarding", "<details>", "## Changelog", "[INT-1913]", "[SUB-1914]")
	if !strings.Contains(minorNotes, "compare/v4.11.0...v4.12.0") {
		t.Errorf("expected compare link v4.11.0...v4.12.0 in v4.12.0 notes")
	}

	v413Notes, vErr := runRenderReleaseNotes(t, repoRoot, "v4.13.0")
	if vErr != nil {
		t.Fatalf("render v4.13.0 release notes failed: %v (%s)", vErr, v413Notes)
	}
	assertOrderedSubstrings(t, v413Notes, "> 11-Language Polyglot Architectural Obligation Proofs", "architecture_boundaries", "<details>", "## Changelog", "compare/v4.12.1...v4.13.0")

	v4131Notes, p1Err := runRenderReleaseNotes(t, repoRoot, "v4.13.1")
	if p1Err != nil {
		t.Fatalf("render v4.13.1 release notes failed: %v (%s)", p1Err, v4131Notes)
	}
	assertOrderedSubstrings(t, v4131Notes, "> Zero-Ceremony Multi-Agent Onboarding Invisibility", "<summary><strong>Verified 2PC Commit Ledger</strong></summary>", "\n## Changelog\n", "* 7ad6bc17eaad5266d9488ba80ba713ea0855bc92 [SUB-2133]", "compare/v4.13.0...v4.13.1", "Included <code>v4.13.0</code> Milestone Features")

	v414Notes, v414Err := runRenderReleaseNotes(t, repoRoot, "v4.14.0")
	if v414Err != nil {
		t.Fatalf("render v4.14.0 release notes failed: %v (%s)", v414Err, v414Notes)
	}
	assertOrderedSubstrings(
		t,
		v414Notes,
		"> 5-Provider Ambient Lifecycle Parity",
		"brew install --cask mgantlett/tap/nomos",
		"<summary><strong>Verified 2PC Commit Ledger</strong></summary>",
		"\n## Changelog\n",
		"\n### Intent & Specifications\n",
		"\n### CLI, Distribution & Hooks\n",
		"[SUB-2188]",
		"\n### Verification & Quality Gates\n",
		"\n### Task, Memory & Session\n",
		"\n### Release & Packaging\n",
		"compare/v4.13.2...v4.14.0",
	)

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

// TestReleaseNotesSubpackageScopeRouting verifies CLI/hook subpackages (e.g. hookcmd) and verify subpackages
// (e.g. xray) classify into their canonical sections rather than falling through to Release & Packaging.
func TestReleaseNotesSubpackageScopeRouting(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	v424Notes, v424Err := runRenderReleaseNotes(t, repoRoot, "v4.24.0")
	if v424Err != nil {
		t.Fatalf("render v4.24.0 release notes failed: %v (%s)", v424Err, v424Notes)
	}
	assertOrderedSubstrings(
		t,
		v424Notes,
		"> Add polyglot design-slop",
		"<summary><strong>Verified 2PC Commit Ledger</strong></summary>",
		"\n## Changelog\n",
		"\n### Intent & Specifications\n",
		"[INT-2126]",
		"\n### CLI, Distribution & Hooks\n",
		"* 4a4f2b76fbd252492555de63ae253e81f3d8e02a [SUB-3201]",
		"* 088d565dbec99d315d17c2715b7a9aaa83c3f02e [SUB-3235]",
		"\n### Verification & Quality Gates\n",
		"[SUB-3241]",
		"\n### Release & Packaging\n",
		"[SUB-3202]",
		"compare/v4.23.0...v4.24.0",
	)

	v4221Notes, v4221Err := runRenderReleaseNotes(t, repoRoot, "v4.22.1")
	if v4221Err != nil {
		t.Fatalf("render v4.22.1 release notes failed: %v (%s)", v4221Err, v4221Notes)
	}
	assertOrderedSubstrings(
		t,
		v4221Notes,
		"<summary><strong>Verified 2PC Commit Ledger</strong></summary>",
		"\n## Changelog\n",
		"\n### CLI, Distribution & Hooks\n",
		"\n### Verification & Quality Gates\n",
		"[SUB-3169]",
		"\n### Release & Packaging\n",
	)
}

// @verifies TC-REL-016
func TestPolyglotDistributionManifestContracts(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	rawGr, err := os.ReadFile(filepath.Join(repoRoot, ".goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read .goreleaser.yaml: %v", err)
	}
	cfg, err := decodeStrictGoReleaserV2(rawGr)
	if err != nil {
		t.Fatalf("decode .goreleaser.yaml: %v", err)
	}
	if len(cfg.DockersV2) != 1 || cfg.DockersV2[0].Images[0] != "ghcr.io/mgantlett/nomos" {
		t.Fatalf("expected dockers_v2 ghcr.io/mgantlett/nomos target, got %+v", cfg.DockersV2)
	}
	if len(cfg.Scoops) != 1 || cfg.Scoops[0].Repository.Name != "scoop-bucket" || len(cfg.Winget) != 1 || cfg.Winget[0].PackageIdentifier != "mgantlett.nomos" {
		t.Fatalf("expected scoops (scoop-bucket) and winget (mgantlett.nomos) targets, got scoops=%+v winget=%+v", cfg.Scoops, cfg.Winget)
	}
	if len(cfg.NFPMs) != 1 || cfg.NFPMs[0].PackageName != "nomos" || len(cfg.NFPMs[0].Formats) != 3 {
		t.Fatalf("expected nfpms Linux deb/rpm/apk package target, got %+v", cfg.NFPMs)
	}
	for _, rel := range []struct {
		file  string
		token string
	}{
		{".pre-commit-hooks.yaml", "id: nomos-verify"},
		{"flake.nix", "packaging/nix/hashes.json"},
		{"packaging/nix/hashes.json", "x86_64-linux"},
		{"Dockerfile", "FROM scratch"},
		{"action.yml", "icon: 'shield'"},
	} {
		data, rErr := os.ReadFile(filepath.Join(repoRoot, rel.file))
		if rErr != nil || !strings.Contains(string(data), rel.token) {
			t.Fatalf("manifest %s missing %q (err=%v)", rel.file, rel.token, rErr)
		}
	}
}
