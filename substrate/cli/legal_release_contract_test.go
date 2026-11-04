package cli

import (
	"os"
	"path/filepath"
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
			name:    "TC-REL-009 release workflow runs tests and 42 DoD gates before GoReleaser",
			relPath: filepath.Join(".github", "workflows", "release.yaml"),
			mustHave: []string{
				"go-version-file: 'substrate/go.mod'",
				"go -C substrate test ./...",
				"verify --all --strict",
				"verify proof",
			},
			forbid: []string{"go-version: 'stable'"},
		},
		{
			name:    "TC-REL-010 uninstall script purges canonical NOMOS_HOME directory",
			relPath: "uninstall.sh",
			mustHave: []string{
				"NOMOS_HOME:-$HOME/" + constants.DirNomosHome,
			},
			forbid: []string{"XDG_DATA_HOME"},
		},
		{
			name:    "TC-REL-011 bug report template excludes sovereign plugins and includes Windsurf",
			relPath: filepath.Join(".github", "ISSUE_TEMPLATE", "01_bug_report.yml"),
			mustHave: []string{"Windsurf", "nomos hook / nomos capabilities"},
			forbid:   []string{"nomos parity", "Nomos v4.0.0"},
		},
		{
			name:    "TC-REL-012 README links arXiv:2610.32285 and deterministic @main go install",
			relPath: "README.md",
			mustHave: []string{
				"CGO_ENABLED=0 go install github.com/mgantlett/nomos/substrate/cmd/nomos@main",
				"https://arxiv.org/abs/2610.32285",
				"https://github.com/TraceLite-AI/TraceTrial-Bench",
			},
		},
		{
			name:    "TC-REL-013 commit_scope contains zero sovereign UI directory special-cases",
			relPath: filepath.Join("substrate", "cli", "commit_scope.go"),
			forbid:  []string{"cockpit-ui", "cockpit"},
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
