package vscope

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeReleaseFixtureFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", full, err)
	}
}

func seedAlignedReleaseRepo(t *testing.T) (string, *VerifyContext) {
	t.Helper()
	root := t.TempDir()
	writeReleaseFixtureFile(t, root, ".goreleaser.yaml", `version: 2
builds:
  - id: nomos
    ldflags:
      - -s -w -X github.com/mgantlett/nomos/substrate/cli.RootCmdVersion={{ .Version }}
archives:
  - name_template: '{{ .ProjectName }}_{{ title .Os }}_{{ .Arch }}'
checksum:
  name_template: 'checksums.txt'
homebrew_casks:
  - name: nomos
    repository:
      token: '{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}'
    skip_upload: '{{ if index .Env "HOMEBREW_TAP_GITHUB_TOKEN" }}auto{{ else }}true{{ end }}'
aurs:
  - name: nomos-bin
    private_key: '{{ .Env.AUR_KEY }}'
    skip_upload: '{{ if index .Env "AUR_KEY" }}auto{{ else }}true{{ end }}'
`)
	writeReleaseFixtureFile(t, root, "substrate/cli/root.go", "package cli\nvar RootCmdVersion = \"4.16.2\"\n")
	writeReleaseFixtureFile(t, root, "CHANGELOG.md", "# Changelog\n\n## [v4.16.2] - 2026-11-11\n\n- Clean release entry - verified.\n")
	writeReleaseFixtureFile(t, root, "substrate/cli/templates/nomos.yaml", "# yaml-language-server: $schema=https://raw.githubusercontent.com/mgantlett/nomos/v4.16.2/nomos.schema.json\n")
	writeReleaseFixtureFile(t, root, "packaging/aur/PKGBUILD", "pkgname=nomos-bin\npkgver=4.16.2\npkgrel=1\n")
	writeReleaseFixtureFile(t, root, "nomos.yaml", "project: nomos\ndefault_branch: main\ngovernance:\n  boundary:\n    enforce: true\n    prohibited_packages:\n      - \"pkg/sovereign\"\n  quality_gates:\n    strict_mode: true\n")

	ctx := &VerifyContext{RepoRoot: root, WorktreePath: root, Strict: true}
	return root, ctx
}

// @verifies TC-SYNC-022
func TestEvaluatePreTagReleaseAirlock_GoReleaserAndSemverParity(t *testing.T) {
	root, ctx := seedAlignedReleaseRepo(t)
	if msg, _ := EvaluatePreTagReleaseAirlock(ctx, root); msg != "" {
		t.Fatalf("expected aligned release repo to pass airlock, got: %s", msg)
	}

	// Broken Go template syntax in .goreleaser.yaml must fail closed.
	writeReleaseFixtureFile(t, root, ".goreleaser.yaml", "version: 2\narchives:\n  - name_template: '{{ .ProjectName '\n")
	if msg, _ := EvaluatePreTagReleaseAirlock(ctx, root); !strings.Contains(msg, "invalid Go template") {
		t.Fatalf("expected invalid Go template rejection, got: %q", msg)
	}

	// Non-secret-only token template (e.g. index .Env on repository.token) must fail closed.
	writeReleaseFixtureFile(t, root, ".goreleaser.yaml", "version: 2\nhomebrew_casks:\n  - name: nomos\n    repository:\n      token: '{{ index .Env \"HOMEBREW_TAP_GITHUB_TOKEN\" }}'\n")
	if msg, _ := EvaluatePreTagReleaseAirlock(ctx, root); !strings.Contains(msg, "secret-only template syntax") {
		t.Fatalf("expected secret-only template rejection, got: %q", msg)
	}

	// Restore valid .goreleaser.yaml and inject PKGBUILD version drift.
	root2, ctx2 := seedAlignedReleaseRepo(t)
	writeReleaseFixtureFile(t, root2, "packaging/aur/PKGBUILD", "pkgname=nomos-bin\npkgver=4.16.0\npkgrel=1\n")
	if msg, _ := EvaluatePreTagReleaseAirlock(ctx2, root2); !strings.Contains(msg, "PKGBUILD pkgver=4.16.0") {
		t.Fatalf("expected PKGBUILD semver pin drift rejection, got: %q", msg)
	}

	// Inject CHANGELOG.md top version drift.
	root3, ctx3 := seedAlignedReleaseRepo(t)
	writeReleaseFixtureFile(t, root3, "CHANGELOG.md", "# Changelog\n\n## [v4.16.1] - 2026-11-11\n")
	if msg, _ := EvaluatePreTagReleaseAirlock(ctx3, root3); !strings.Contains(msg, "CHANGELOG.md top release v4.16.1") {
		t.Fatalf("expected CHANGELOG.md semver pin drift rejection, got: %q", msg)
	}
}

// @verifies TC-SYNC-023
func TestEvaluatePreTagReleaseAirlock_ChangelogSanitization(t *testing.T) {
	root, ctx := seedAlignedReleaseRepo(t)
	homeLeak := "/" + "home/alice/repo/app.go"
	writeReleaseFixtureFile(t, root, "CHANGELOG.md", "# Changelog\n\n## [v4.16.2] - 2026-11-11\n\n- Fixed "+homeLeak+" path.\n")
	if msg, _ := EvaluatePreTagReleaseAirlock(ctx, root); !strings.Contains(msg, "path prefix") {
		t.Fatalf("expected host home path leak in CHANGELOG.md to fail closed, got: %q", msg)
	}

	writeReleaseFixtureFile(t, root, "CHANGELOG.md", "# Changelog\n\n## [v4.16.2] - 2026-11-11\n\n- Ported helper from pkg/sovereign/core.\n")
	if msg, _ := EvaluatePreTagReleaseAirlock(ctx, root); !strings.Contains(msg, "prohibited boundary package") {
		t.Fatalf("expected prohibited boundary package in CHANGELOG.md to fail closed, got: %q", msg)
	}

	writeReleaseFixtureFile(t, root, "CHANGELOG.md", "# Changelog\n\n## [v4.16.2] - 2026-11-11\n\n- Added release airlock \u2014 verified.\n")
	if msg, _ := EvaluatePreTagReleaseAirlock(ctx, root); !strings.Contains(msg, "em-dash") {
		t.Fatalf("expected Unicode em-dash in strict CHANGELOG.md to fail closed, got: %q", msg)
	}

	writeReleaseFixtureFile(t, root, "CHANGELOG.md", "# Changelog\n\n## [v4.16.2] - 2026-11-11\n\n- Enforced orphaned @verifies check while keeping `@verifies` valid.\n")
	if msg, _ := EvaluatePreTagReleaseAirlock(ctx, root); !strings.Contains(msg, "@mention") {
		t.Fatalf("expected bare @verifies in strict CHANGELOG.md to fail closed, got: %q", msg)
	}
}
