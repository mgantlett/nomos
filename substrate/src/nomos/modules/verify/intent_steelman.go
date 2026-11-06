// Package verify implements Intent Steel-Man frontmatter, structural, and NCTE-100 validation (Law 4 & Law 6).
package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/ncte"
	"gopkg.in/yaml.v3"
)

// Delimiter and canonical document type constants for Intent Steel-Man verification.
const (
	frontmatterOpenDelimiter  = "---\n"
	frontmatterCloseDelimiter = "\n---\n"
	docTypeSpec               = "spec"
)

var (
	// isoDatePattern validates canonical YYYY-MM-DD frontmatter date strings.
	isoDatePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	// semverPattern validates canonical X.Y.Z SemVer strings in YAML frontmatter.
	semverPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[A-Za-z0-9.-]+)?$`)
	// specTestVectorsHeadingRe matches required H2 test vector sections in type: spec documents.
	specTestVectorsHeadingRe = regexp.MustCompile(`(?m)^##\s+.*(?i:Test Vector|Test Matrix|Verification Matrix|Verification Vector|Compliance Vector|Table-Driven)`)

	validIntentPlanes = map[string]bool{
		"intent":    true,
		"substrate": true,
	}
	validIntentStatuses = map[string]bool{
		"active":     true,
		"superseded": true,
		"deprecated": true,
		"archived":   true,
	}
	validIntentClassifications = map[string]bool{
		"public":    true,
		"internal":  true,
		"sovereign": true,
	}
	validIntentDocTypes = map[string]bool{
		docTypeSpec:    true,
		"handbook":     true,
		"architecture": true,
		"research":     true,
	}
)

// intentFrontmatter models the canonical 10-key YAML frontmatter schema for Intent documents.
type intentFrontmatter struct {
	Authors        []string `yaml:"authors"`
	Classification string   `yaml:"classification"`
	Date           string   `yaml:"date"`
	LastUpdated    string   `yaml:"last_updated"`
	Plane          string   `yaml:"plane"`
	SSoTRepository string   `yaml:"ssot_repository"`
	Status         string   `yaml:"status"`
	Title          string   `yaml:"title"`
	Type           string   `yaml:"type"`
	Version        string   `yaml:"version"`
}

// intentSteelmanOptions configures baseline vs strict Steel-Man and NCTE-100 enforcement.
type intentSteelmanOptions struct {
	RequireFrontmatter bool
	StrictStandard     bool
}

// validateIntentDocuments audits Markdown files under intentDir using baseline frontmatter rules.
func validateIntentDocuments(intentDir string, requireFrontmatter bool) (int, []string) {
	return validateIntentDocumentsWithOptions(intentDir, intentSteelmanOptions{
		RequireFrontmatter: requireFrontmatter,
	})
}

// validateIntentDocumentsWithOptions walks intentDir and evaluates each Markdown document against opts.
func validateIntentDocumentsWithOptions(intentDir string, opts intentSteelmanOptions) (int, []string) {
	var docCount int
	var issues []string

	_ = filepath.Walk(intentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		docCount++
		rel, rErr := filepath.Rel(intentDir, path)
		if rErr != nil {
			rel = filepath.Base(path)
		}
		rel = filepath.ToSlash(rel)
		if docErr := auditSingleIntentMarkdownWithOptions(rel, path, opts); docErr != "" {
			issues = append(issues, fmt.Sprintf("%s: %s", rel, docErr))
		}
		return nil
	})
	return docCount, issues
}

// auditSingleIntentMarkdownWithOptions validates frontmatter, balanced fences, structure, and NCTE-100 prose.
func auditSingleIntentMarkdownWithOptions(rel, path string, opts intentSteelmanOptions) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("unreadable document (%v)", err)
	}
	normalized := strings.ReplaceAll(string(raw), "\r\n", "\n")
	fm, body, hasFM, parseErr := extractAndParseIntentFrontmatter(normalized, opts.RequireFrontmatter)
	if parseErr != "" {
		return parseErr
	}
	if hasUnclosedCodeFence(body) {
		return "unclosed Markdown fenced code block detected"
	}
	if !hasFM || !opts.StrictStandard {
		return ""
	}
	return evaluateStrictIntentStandard(rel, raw, normalized, fm, body)
}

// extractAndParseIntentFrontmatter isolates and unmarshals leading YAML frontmatter blocks.
func extractAndParseIntentFrontmatter(normalized string, requireFrontmatter bool) (intentFrontmatter, string, bool, string) {
	var fm intentFrontmatter
	if !strings.HasPrefix(normalized, frontmatterOpenDelimiter) {
		if requireFrontmatter {
			return fm, normalized, false, "missing leading YAML frontmatter delimiter (---)"
		}
		return fm, normalized, false, ""
	}
	offset := len(frontmatterOpenDelimiter)
	endIdx := strings.Index(normalized[offset:], frontmatterCloseDelimiter)
	if endIdx < 0 {
		return fm, "", false, "missing closing YAML frontmatter delimiter (---)"
	}
	fmRaw := normalized[offset : offset+endIdx]
	body := normalized[offset+endIdx+len(frontmatterCloseDelimiter):]
	if yErr := yaml.Unmarshal([]byte(fmRaw), &fm); yErr != nil {
		return fm, body, false, fmt.Sprintf("malformed YAML frontmatter (%v)", yErr)
	}
	if baseIssue := validateBaseIntentFrontmatter(fm, requireFrontmatter); baseIssue != "" {
		return fm, body, true, baseIssue
	}
	return fm, body, true, ""
}

// validateBaseIntentFrontmatter checks core title, plane, and status fields.
func validateBaseIntentFrontmatter(fm intentFrontmatter, requireFrontmatter bool) string {
	if strings.TrimSpace(fm.Title) == "" {
		return "YAML frontmatter missing required non-empty 'title'"
	}
	if fm.Plane != "" || requireFrontmatter {
		plane := strings.ToLower(strings.TrimSpace(fm.Plane))
		if !validIntentPlanes[plane] {
			return fmt.Sprintf("invalid frontmatter plane %q (expected intent or substrate)", fm.Plane)
		}
	}
	if fm.Status != "" || requireFrontmatter {
		status := strings.ToLower(strings.TrimSpace(fm.Status))
		if !validIntentStatuses[status] {
			return fmt.Sprintf("invalid frontmatter status %q (expected active, superseded, deprecated, or archived)", fm.Status)
		}
	}
	return ""
}

// evaluateStrictIntentStandard enforces dash hygiene, 10-key frontmatter, spec anatomy, and NCTE-100 rules.
func evaluateStrictIntentStandard(rel string, raw []byte, normalized string, fm intentFrontmatter, body string) string {
	if strings.ContainsRune(normalized, '\u2014') || strings.ContainsRune(normalized, '\u2013') {
		return "contains prohibited Unicode em-dash or en-dash; use ASCII hyphens or colons"
	}
	if fmIssue := validateStrictIntentFrontmatter(fm); fmIssue != "" {
		return fmIssue
	}
	if structIssue := validateStrictSpecStructure(rel, fm, body); structIssue != "" {
		return structIssue
	}
	return auditIntentMarkdownNCTE(rel, raw)
}

// validateStrictIntentFrontmatter verifies all 10 statutory frontmatter keys in strict mode.
func validateStrictIntentFrontmatter(fm intentFrontmatter) string {
	if len(fm.Authors) == 0 {
		return "YAML frontmatter missing required non-empty 'authors' list"
	}
	classVal := strings.ToLower(strings.TrimSpace(fm.Classification))
	if !validIntentClassifications[classVal] {
		return fmt.Sprintf("invalid frontmatter classification %q (expected public, internal, or sovereign)", fm.Classification)
	}
	if !isoDatePattern.MatchString(strings.TrimSpace(fm.Date)) {
		return fmt.Sprintf("invalid frontmatter date %q (expected YYYY-MM-DD)", fm.Date)
	}
	if !isoDatePattern.MatchString(strings.TrimSpace(fm.LastUpdated)) {
		return fmt.Sprintf("invalid frontmatter last_updated %q (expected YYYY-MM-DD)", fm.LastUpdated)
	}
	if strings.TrimSpace(fm.SSoTRepository) == "" {
		return "YAML frontmatter missing required non-empty 'ssot_repository'"
	}
	docType := strings.ToLower(strings.TrimSpace(fm.Type))
	if !validIntentDocTypes[docType] {
		return fmt.Sprintf("invalid frontmatter type %q (expected spec, handbook, architecture, or research)", fm.Type)
	}
	if !semverPattern.MatchString(strings.TrimSpace(fm.Version)) {
		return fmt.Sprintf("invalid frontmatter version %q (expected SemVer X.Y.Z)", fm.Version)
	}
	return ""
}

// validateStrictSpecStructure enforces _spec.md naming under specs/ and required Test Vector sections.
func validateStrictSpecStructure(rel string, fm intentFrontmatter, body string) string {
	docType := strings.ToLower(strings.TrimSpace(fm.Type))
	if strings.Contains("/"+rel, "/specs/") {
		if !strings.HasSuffix(rel, "_spec.md") {
			return "specification under specs/ must use canonical '_spec.md' suffix (Law 6)"
		}
		if docType != docTypeSpec {
			return "specification under specs/ must declare frontmatter 'type: spec'"
		}
	}
	if docType == docTypeSpec && !specTestVectorsHeadingRe.MatchString(body) {
		return "specification of type 'spec' missing required '## ... Table-Driven Test Vectors/Matrix' section (Law 2)"
	}
	return ""
}

// auditIntentMarkdownNCTE executes the deterministic NCTE-100 prose linter against raw Markdown bytes.
func auditIntentMarkdownNCTE(rel string, raw []byte) string {
	rep, err := ncte.NewEngine(nil).LintMarkdown(rel, raw)
	if err != nil {
		return fmt.Sprintf("NCTE-100 lint failure (%v)", err)
	}
	if !rep.Passed && len(rep.Diagnostics) > 0 {
		first := rep.Diagnostics[0]
		return fmt.Sprintf("NCTE-100 violation at line %d [%s]: %s (%d total)",
			first.Line, first.Code, first.Message, rep.ViolationsCount)
	}
	return ""
}

// hasUnclosedCodeFence detects unbalanced triple-or-more backtick fences in Markdown bodies.
func hasUnclosedCodeFence(body string) bool {
	inFence := false
	fenceLen := 0
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		ticks := countLeadingBackticks(trimmed)
		if ticks < 3 {
			continue
		}
		rest := strings.TrimSpace(trimmed[ticks:])
		if !inFence {
			inFence = true
			fenceLen = ticks
		} else if ticks >= fenceLen && rest == "" {
			inFence = false
			fenceLen = 0
		}
	}
	return inFence
}

// countLeadingBackticks counts consecutive leading backtick runes on a trimmed line.
func countLeadingBackticks(s string) int {
	n := 0
	for n < len(s) && s[n] == '`' {
		n++
	}
	return n
}
