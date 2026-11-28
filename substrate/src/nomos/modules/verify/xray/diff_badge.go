// Package xray implements PR diff-scoped candidate selection and shareable score badges.
package xray

import (
	"fmt"
	"html"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
)

const xrayPortalURL = "https://sophialabs.ca/xray"

func collectDiffCandidates(absRoot, baseRef string) ([]scanCandidate, int, error) {
	wtRoot, err := workspace.FindWorkingTreeRoot(absRoot)
	if err != nil || wtRoot == "" {
		return nil, 0, fmt.Errorf("--diff requires a Git repository working tree")
	}
	if verifyErr := workspace.SafeGitCommand(absRoot, "rev-parse", "--verify", "--quiet", baseRef+"^{commit}").Run(); verifyErr != nil {
		return nil, 0, fmt.Errorf("unknown Git base ref %q for --diff", baseRef)
	}
	relPaths, err := collectGitDiffPaths(absRoot, baseRef)
	if err != nil {
		return nil, 0, err
	}
	cands := buildFilteredCandidates(absRoot, relPaths)
	return cands, len(cands), nil
}

func collectGitDiffPaths(absRoot, baseRef string) ([]string, error) {
	seen := make(map[string]bool)
	queries := [][]string{
		{"diff", "-z", "--name-only", "--diff-filter=ACMR", baseRef + "...HEAD", "--", "."},
		{"diff", "-z", "--name-only", "--diff-filter=ACMR", "HEAD", "--", "."},
		{"ls-files", "-z", "--others", "--exclude-standard", "--", "."},
	}
	for i, args := range queries {
		out, err := workspace.SafeGitCommand(absRoot, args...).Output()
		if err != nil {
			if i == 0 {
				return nil, fmt.Errorf("git diff against %q failed: %w", baseRef, err)
			}
			continue
		}
		for _, p := range parseUniqueGitPaths(out) {
			seen[p] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths, nil
}

func buildFilteredCandidates(absRoot string, relPaths []string) []scanCandidate {
	cands := make([]scanCandidate, 0, len(relPaths))
	for _, relSlash := range relPaths {
		base := filepath.Base(relSlash)
		if hasSkippedDirSegment(relSlash) || isBundledArtifactFileName(base) || isBundledAssetRelPath(relSlash) {
			continue
		}
		lang := treesitter.DetectLanguage(base)
		if !codeLanguages[lang] {
			continue
		}
		cands = append(cands, scanCandidate{
			relSlash: relSlash,
			lang:     lang,
			absPath:  filepath.Join(absRoot, filepath.FromSlash(relSlash)),
		})
	}
	return cands
}

// FormatBadgeMarkdown returns a copy-pasteable Shields.io Markdown badge for README or PR sharing.
func FormatBadgeMarkdown(rep *XrayReport) string {
	if rep == nil {
		return ""
	}
	grade := normalizedGrade(rep.Grade)
	value := fmt.Sprintf("%d/100 (%s)", rep.HangoverScore, grade)
	encoded := url.PathEscape(strings.ReplaceAll(value, "-", "--"))
	color := strings.TrimPrefix(badgeColorForGrade(grade), "#")
	return fmt.Sprintf("[![Nomos X-Ray](https://img.shields.io/badge/Nomos_X--Ray-%s-%s?style=flat-square)](%s)", encoded, color, xrayPortalURL)
}

// FormatBadgeSVG renders a deterministic standalone flat-square SVG badge for the XrayReport.
func FormatBadgeSVG(rep *XrayReport) string {
	if rep == nil {
		return ""
	}
	grade := normalizedGrade(rep.Grade)
	label := "Nomos X-Ray"
	value := fmt.Sprintf("%d/100 (%s)", rep.HangoverScore, grade)
	color := badgeColorForGrade(grade)
	leftW := 92
	rightW := len(value)*7 + 16
	totalW := leftW + rightW
	leftCenter := leftW * 5
	rightCenter := (leftW + rightW/2) * 10
	escValue := html.EscapeString(value)
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s">`+
		`<title>%s: %s</title>`+
		`<g shape-rendering="crispEdges">`+
		`<rect width="%d" height="20" fill="#1e293b"/>`+
		`<rect x="%d" width="%d" height="20" fill="%s"/>`+
		`</g>`+
		`<g fill="#ffffff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="110">`+
		`<text x="%d" y="140" transform="scale(.1)" fill="#ffffff">%s</text>`+
		`<text x="%d" y="140" transform="scale(.1)" fill="#ffffff" font-weight="bold">%s</text>`+
		`</g></svg>`,
		totalW, label, escValue, label, escValue,
		leftW, leftW, rightW, color,
		leftCenter, label, rightCenter, escValue,
	)
}

func normalizedGrade(grade string) string {
	g := strings.ToUpper(strings.TrimSpace(grade))
	if g == "" {
		return "A"
	}
	return g
}

func badgeColorForGrade(grade string) string {
	switch {
	case strings.HasPrefix(grade, "A"):
		return "#10b981"
	case strings.HasPrefix(grade, "B"):
		return "#14b8a6"
	case strings.HasPrefix(grade, "C"):
		return "#f59e0b"
	default:
		return "#ef4444"
	}
}
