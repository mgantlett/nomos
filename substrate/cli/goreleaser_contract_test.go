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
	Version     int                       `yaml:"version"`
	ProjectName string                    `yaml:"project_name"`
	Before      strictGoReleaserBefore    `yaml:"before"`
	Builds      []strictGoReleaserBuild   `yaml:"builds"`
	Archives    []strictGoReleaserArchive `yaml:"archives"`
	Checksum    strictGoReleaserChecksum  `yaml:"checksum"`
	AURs        []strictGoReleaserAUR     `yaml:"aurs"`
	Changelog   strictGoReleaserChangelog `yaml:"changelog"`
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

func TestReleaseWorkflowPatchNotesRollup(t *testing.T) {
	bashBin, err := exec.LookPath("bash")
	if err != nil {
		return
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	relYaml, err := os.ReadFile(filepath.Join(repoRoot, ".github", "workflows", "release.yaml"))
	if err != nil {
		t.Fatalf("read release.yaml: %v", err)
	}
	if !strings.Contains(string(relYaml), "Included `%s` Milestone Features") {
		t.Fatalf("expected release.yaml to roll up parent vX.Y.0 milestone features on patch releases")
	}

	tmpOut := t.TempDir()
	script := `set -euo pipefail
TAG="$1"
extract_section() {
  awk -v tag="[$1]" '
    index($0, "## " tag) == 1 { flag=1; next }
    flag && /^## \[/ { exit }
    flag { print }
  ' CHANGELOG.md | sed '/^---$/d'
}
extract_section "$TAG" > "$RUNNER_TEMP/release-notes.md"
if printf '%s\n' "$TAG" | grep -qE '^v[0-9]+\.[0-9]+\.[1-9][0-9]*$'; then
  MINOR_BASE="${TAG%.*}.0"
  extract_section "$MINOR_BASE" > "$RUNNER_TEMP/minor-notes.md"
  if [ -s "$RUNNER_TEMP/minor-notes.md" ]; then
    {
      printf '\n---\n\n## Included ` + "`%s`" + ` Milestone Features\n\n' "$MINOR_BASE"
      cat "$RUNNER_TEMP/minor-notes.md"
    } >> "$RUNNER_TEMP/release-notes.md"
  fi
fi`

	cmd := exec.Command(bashBin, "-c", script, "release-notes-test", "v4.10.1")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "RUNNER_TEMP="+tmpOut)
	if out, cErr := cmd.CombinedOutput(); cErr != nil {
		t.Fatalf("release notes extraction failed: %v (%s)", cErr, string(out))
	}
	notesBytes, rErr := os.ReadFile(filepath.Join(tmpOut, "release-notes.md"))
	if rErr != nil {
		t.Fatalf("read generated release-notes.md: %v", rErr)
	}
	notes := string(notesBytes)
	for _, want := range []string{"GoReleaser v2 `config.AUR` Schema Parity", "Included `v4.10.0` Milestone Features", ".opencode/plugins/nomos.ts"} {
		if !strings.Contains(notes, want) {
			t.Errorf("expected patch release notes for v4.10.1 to contain %q, got:\n%s", want, notes)
		}
	}
}
