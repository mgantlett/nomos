// @intent task_lifecycle_cli_spec.md
package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

type repoContractCase struct {
	name     string
	relPath  string
	mustHave []string
	forbid   []string
}

func legalContractCases() []repoContractCase {
	return []repoContractCase{
		{
			name:    "TC-REL-001 verbatim SPDX Apache-2.0 LICENSE with Appendix",
			relPath: "LICENSE",
			mustHave: []string{
				"Apache License",
				"Version 2.0, January 2004",
				"END OF TERMS AND CONDITIONS",
				"APPENDIX: How to apply the Apache License to your work.",
				"Copyright [yyyy] [name of copyright owner]",
			},
		},
		{
			name:    "TC-REL-006 corporate NOTICE trademark shield and logo carve-out without patent disclosure",
			relPath: "NOTICE",
			mustHave: []string{
				"Copyright 2026 Gantlett Systems Inc. (SophiaLabs)",
				"Section 6 of the Apache License, Version 2.0",
				"\"Nomos Mneme\"",
				"sophialabs_logo.png",
				"nomos_logo.png",
				"excluded from the copyright license",
			},
			forbid: []string{"Patent Pending", "patent pending", "USPTO"},
		},
		{
			name:    "TC-REL-005 THIRD_PARTY_NOTICES covers direct Go dependencies",
			relPath: "THIRD_PARTY_NOTICES.md",
			mustHave: []string{
				"github.com/charmbracelet/lipgloss",
				"github.com/muesli/termenv",
				"github.com/spf13/cobra",
				"github.com/tetratelabs/wazero",
				"gopkg.in/yaml.v3",
				"modernc.org/sqlite",
				"golang.org/x/sys",
			},
		},
		{
			name:     "TC-REL-007 CONTRIBUTING and PR template codify DCO provenance",
			relPath:  "CONTRIBUTING.md",
			mustHave: []string{"Developer Certificate of Origin", "Signed-off-by:", "Section 5"},
		},
		{
			name:     "TC-REL-007 PR template includes DCO provenance check",
			relPath:  filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
			mustHave: []string{"Signed-off-by:", "DCO"},
		},
	}
}

func operationalSurfaceContractCases() []repoContractCase {
	return []repoContractCase{
		{
			name:    "TC-REL-005 GoReleaser archive bundles notices and verifies modules read-only",
			relPath: ".goreleaser.yaml",
			mustHave: []string{
				"go -C substrate mod verify",
				"- LICENSE",
				"- NOTICE",
				"- THIRD_PARTY_NOTICES.md",
				"- README.md",
			},
			forbid: []string{"mod tidy"},
		},
		{
			name:    "TC-REL-009 unified CI workflow runs tests and 42 DoD gates before GoReleaser",
			relPath: filepath.Join(".github", "workflows", "ci.yaml"),
			mustHave: []string{
				"go-version-file: 'substrate/go.mod'",
				"go -C substrate test ./...",
				"verify --all --strict",
				"verify proof",
				"needs: verify",
				"id-token: write",
				"registry.npmjs.org",
				"registry.modelcontextprotocol.io",
			},
			forbid: []string{"go-version: 'stable'"},
		},
		{
			name:    "TC-REL-010 uninstall script purges canonical NOMOS_HOME directory and is POSIX sh tolerant",
			relPath: "uninstall.sh",
			mustHave: []string{
				"NOMOS_HOME:-$HOME/" + constants.DirNomosHome,
				"(set -o pipefail) 2>/dev/null && set -o pipefail",
			},
			forbid: []string{"XDG_DATA_HOME", "set -euo pipefail"},
		},
		{
			name:    "TC-REL-010b install script is POSIX sh tolerant and guides Windows shells to WSL2 or GitHub Releases",
			relPath: "install.sh",
			mustHave: []string{
				"(set -o pipefail) 2>/dev/null && set -o pipefail",
				"mingw*|msys*|cygwin*",
				"https://github.com/mgantlett/nomos/releases/latest",
			},
			forbid: []string{"set -euo pipefail"},
		},
		{
			name:     "TC-REL-011 bug report template excludes sovereign plugins and includes Windsurf",
			relPath:  filepath.Join(".github", "ISSUE_TEMPLATE", "01_bug_report.yml"),
			mustHave: []string{"Windsurf", "nomos hook / nomos capabilities"},
			forbid:   []string{"nomos parity", "Nomos v4.0.0"},
		},
		{
			name:    "TC-REL-012 README links arXiv:2610.32285, receipt-gated episodic memory, and pure open-core boundary",
			relPath: "README.md",
			mustHave: []string{
				"CGO_ENABLED=0 go install github.com/mgantlett/nomos/substrate/cmd/nomos@main",
				"https://arxiv.org/abs/2610.32285",
				"https://github.com/TraceLite-AI/TraceTrial-Bench",
				"Receipt-Gated Episodic Memory",
				"Multi-Seat CapBAC Leases & Cross-Repo DAGs",
				"https://sophialabs.ca/sovereign#nomos-code",
				"https://sophialabs.ca/api/xray/badge/mgantlett/nomos.svg",
				"https://sophialabs.ca/system/analytics",
				"intent/assets/nomos_hero_terminal.svg",
				"0/100 Grade A",
			},
			forbid: []string{
				"Autonomous Tier 2 Coding Agent",
				"Cockpit Control Plane",
				"nomos_hero_demo.gif",
			},
		},
		{
			name:    "TC-REL-013 commit_scope contains zero sovereign UI directory special-cases",
			relPath: filepath.Join("substrate", "cli", "commitmsg", "commit_scope.go"),
			forbid:  []string{"cockpit-ui", "cockpit"},
		},
		{
			name:    "TC-REL-014 GoReleaser AUR nomos-bin static binary target",
			relPath: ".goreleaser.yaml",
			mustHave: []string{
				"aurs:",
				"name: nomos-bin",
				"ssh://aur@aur.archlinux.org/nomos-bin.git",
				`install -Dm755 "./nomos" "${pkgdir}/usr/bin/nomos"`,
				`install -Dm644 "./LICENSE" "${pkgdir}/usr/share/licenses/nomos-bin/LICENSE"`,
			},
		},
		{
			name:    "TC-REL-015 Arch/Omarchy packaging/aur/PKGBUILD static binary contract",
			relPath: filepath.Join("packaging", "aur", "PKGBUILD"),
			mustHave: []string{
				"pkgname=nomos-bin",
				"arch=('x86_64' 'aarch64')",
				"license=('Apache-2.0')",
				"options=('!strip' '!debug')",
				"sha256sums_x86_64=",
				"sha256sums_aarch64=",
			},
		},
		{
			name:    "Agent Skill SKILL.md stays a strict Thin Binary Bootstrapper",
			relPath: filepath.Join("skills", "nomos", "SKILL.md"),
			mustHave: []string{
				"name: nomos",
				"Thin Binary Bootstrapper",
				"https://sophialabs.ca/install",
				"nomos xray .",
				"nomos init",
			},
			forbid: []string{"cockpit", "Autonomous Tier 2"},
		},
		{
			name:    "Claude Plugin manifest wires skills directory under Apache-2.0",
			relPath: filepath.Join(".claude-plugin", "plugin.json"),
			mustHave: []string{
				`"name": "nomos"`,
				`"license": "Apache-2.0"`,
				`"skills": "./skills/"`,
			},
			forbid: []string{"cockpit"},
		},
		{
			name:    "Claude Marketplace manifest exposes nomos root plugin",
			relPath: filepath.Join(".claude-plugin", "marketplace.json"),
			mustHave: []string{
				`"name": "nomos"`,
				`"source": "./"`,
			},
			forbid: []string{"cockpit"},
		},
		{
			name:    "llms.txt and npm launcher expose Homebrew, npx, and remote Streamable HTTP MCP",
			relPath: "llms.txt",
			mustHave: []string{
				"brew install --cask mgantlett/tap/nomos",
				"npx --yes @mgantlett/nomos xray .",
				"https://sophialabs.ca/mcp",
				"https://sophialabs.ca/.well-known/openapi.json",
			},
			forbid: []string{"cockpit"},
		},
		{
			name:    "npm launcher package manifest exposes Apache-2.0 CLI wrapper template",
			relPath: filepath.Join("packaging", "npm", "package.json"),
			mustHave: []string{
				`"@mgantlett/nomos"`,
				`"mcpName": "io.github.mgantlett/nomos"`,
				`"Apache-2.0"`,
				`"./bin/nomos-cli.mjs"`,
			},
			forbid: []string{"cockpit", "@sophialabs/nomos"},
		},
		{
			name:    "official MCP registry server.json pairs remote HTTP, npm, and pypi stdio packages",
			relPath: "server.json",
			mustHave: []string{
				`"io.github.mgantlett/nomos"`,
				`"https://sophialabs.ca/mcp"`,
				`"@mgantlett/nomos"`,
				`"nomos-cli"`,
			},
			forbid: []string{"cockpit"},
		},
		{
			name:     "PyPI README declares Official MCP Registry mcp-name ownership marker",
			relPath:  filepath.Join("packaging", "pypi", "README.md"),
			mustHave: []string{"<!-- mcp-name: io.github.mgantlett/nomos -->"},
			forbid:   []string{"cockpit"},
		},
		{
			name:    "Glama MCP directory manifest declares maintainer ownership",
			relPath: "glama.json",
			mustHave: []string{
				`"https://glama.ai/mcp/schemas/server.json"`,
				`"mgantlett"`,
			},
			forbid: []string{"cockpit"},
		},
		{
			name:    "Context7 indexing manifest exposes curated docs and Day-0 agent rules",
			relPath: "context7.json",
			mustHave: []string{
				`"https://context7.com/schema/context7.json"`,
				`"Nomos Open-Core"`,
				`"excludeFolders"`,
				`https://sophialabs.ca/mcp`,
			},
			forbid: []string{"cockpit", "Nomos Harness"},
		},
		{
			name:    "npm launcher script verifies SHA-256 checksums before execution",
			relPath: filepath.Join("packaging", "npm", "bin", "nomos-cli.mjs"),
			mustHave: []string{
				"#!/usr/bin/env node",
				"checksums.txt",
				"createHash('sha256')",
			},
			forbid: []string{"cockpit"},
		},
	}
}

func TestOpenCoreLegalAndReleaseContracts(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	all := append(legalContractCases(), operationalSurfaceContractCases()...)
	for _, tc := range all {
		t.Run(tc.name, func(t *testing.T) {
			raw, readErr := os.ReadFile(filepath.Join(repoRoot, tc.relPath))
			if readErr != nil {
				t.Fatalf("read %s: %v", tc.relPath, readErr)
			}
			body := string(raw)
			for _, want := range tc.mustHave {
				if !strings.Contains(body, want) {
					t.Errorf("%s missing required substring %q", tc.relPath, want)
				}
			}
			for _, bad := range tc.forbid {
				if strings.Contains(body, bad) {
					t.Errorf("%s contains prohibited substring %q", tc.relPath, bad)
				}
			}
		})
	}
}

func TestAURPKGBUILD_SHA256AndPackageExecution(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	pkgbuildPath := filepath.Join(repoRoot, "packaging", "aur", "PKGBUILD")
	raw, err := os.ReadFile(pkgbuildPath)
	if err != nil {
		t.Fatalf("read PKGBUILD: %v", err)
	}
	shaRe := regexp.MustCompile(`sha256sums_(?:x86_64|aarch64)=\('([0-9a-f]{64})'\)`)
	matches := shaRe.FindAllStringSubmatch(string(raw), -1)
	if len(matches) != 2 {
		t.Fatalf("expected 2 pinned 64-hex SHA-256 sums in PKGBUILD, got %d", len(matches))
	}

	bashBin, bErr := exec.LookPath("bash")
	if bErr != nil {
		return
	}
	_ = ensureCompiledHookTestEnv(t)
	srcDir := t.TempDir()
	pkgDir := t.TempDir()

	binBytes, rErr := os.ReadFile(filepath.Join(compiledHookTestBinDir, "nomos"))
	if rErr != nil {
		t.Fatalf("read compiled nomos: %v", rErr)
	}
	if wErr := os.WriteFile(filepath.Join(srcDir, "nomos"), binBytes, 0755); wErr != nil {
		t.Fatalf("stage nomos in srcdir: %v", wErr)
	}
	for _, doc := range []string{"LICENSE", "NOTICE", "THIRD_PARTY_NOTICES.md", "README.md", "CHANGELOG.md"} {
		docBytes, dErr := os.ReadFile(filepath.Join(repoRoot, doc))
		if dErr != nil {
			t.Fatalf("read %s: %v", doc, dErr)
		}
		if wErr := os.WriteFile(filepath.Join(srcDir, doc), docBytes, 0644); wErr != nil {
			t.Fatalf("stage %s: %v", doc, wErr)
		}
	}

	script := `set -euo pipefail; srcdir="$1"; pkgdir="$2"; source "$3"; package`
	cmd := exec.Command(bashBin, "-c", script, "pkgbuild-test", srcDir, pkgDir, pkgbuildPath)
	if out, cErr := cmd.CombinedOutput(); cErr != nil {
		t.Fatalf("PKGBUILD package() execution failed (%v):\n%s", cErr, string(out))
	}

	installedBin := filepath.Join(pkgDir, "usr", "bin", "nomos")
	verOut, vErr := exec.Command(installedBin, "--version").CombinedOutput()
	if vErr != nil {
		t.Fatalf("installed /usr/bin/nomos --version failed (%v): %s", vErr, string(verOut))
	}
	if _, lErr := os.Stat(filepath.Join(pkgDir, "usr", "share", "licenses", "nomos-bin", "LICENSE")); lErr != nil {
		t.Fatalf("expected installed LICENSE under /usr/share/licenses/nomos-bin/LICENSE: %v", lErr)
	}
}

func TestOpenCoreManifestVersionParity(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	pkgRaw, err := os.ReadFile(filepath.Join(repoRoot, "packaging", "npm", "package.json"))
	if err != nil {
		t.Fatalf("read packaging/npm/package.json: %v", err)
	}
	var npmManifest struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		McpName string `json:"mcpName"`
	}
	if err := json.Unmarshal(pkgRaw, &npmManifest); err != nil || npmManifest.Name != "@mgantlett/nomos" || npmManifest.McpName != "io.github.mgantlett/nomos" || npmManifest.Version != RootCmdVersion {
		t.Fatalf("npm manifest drift vs RootCmdVersion %q: err=%v %+v", RootCmdVersion, err, npmManifest)
	}
	pypiRaw, err := os.ReadFile(filepath.Join(repoRoot, "packaging", "pypi", "pyproject.toml"))
	if err != nil || !strings.Contains(string(pypiRaw), `version = "`+RootCmdVersion+`"`) {
		t.Fatalf("pyproject.toml version drift vs RootCmdVersion %q: err=%v", RootCmdVersion, err)
	}
	srvRaw, err := os.ReadFile(filepath.Join(repoRoot, "server.json"))
	if err != nil {
		t.Fatalf("read server.json: %v", err)
	}
	var mcpManifest struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		Packages []struct {
			Identifier string `json:"identifier"`
			Version    string `json:"version"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(srvRaw, &mcpManifest); err != nil || mcpManifest.Version != RootCmdVersion || len(mcpManifest.Packages) < 2 {
		t.Fatalf("server.json parity drift vs RootCmdVersion %q: err=%v %+v", RootCmdVersion, err, mcpManifest)
	}
	for _, pkg := range mcpManifest.Packages {
		if pkg.Version != RootCmdVersion {
			t.Fatalf("server.json package %q version %q != %q", pkg.Identifier, pkg.Version, RootCmdVersion)
		}
	}
}
