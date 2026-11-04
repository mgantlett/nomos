package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCoreLegalAndReleaseContracts(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	cases := []struct {
		name     string
		relPath  string
		mustHave []string
		forbid   []string
	}{
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
			forbid: []string{
				"Patent Pending",
				"patent pending",
				"USPTO",
			},
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
			name:    "TC-REL-005 GoReleaser archive includes LICENSE NOTICE and THIRD_PARTY_NOTICES",
			relPath: ".goreleaser.yaml",
			mustHave: []string{
				"- LICENSE",
				"- NOTICE",
				"- THIRD_PARTY_NOTICES.md",
				"- README.md",
			},
		},
		{
			name:    "TC-REL-007 CONTRIBUTING codifies DCO Signed-off-by provenance",
			relPath: "CONTRIBUTING.md",
			mustHave: []string{
				"Developer Certificate of Origin",
				"Signed-off-by:",
				"Section 5",
			},
		},
		{
			name:    "TC-REL-007 PR template includes DCO provenance check",
			relPath: filepath.Join(".github", "PULL_REQUEST_TEMPLATE.md"),
			mustHave: []string{
				"Signed-off-by:",
				"DCO",
			},
		},
	}

	for _, tc := range cases {
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
