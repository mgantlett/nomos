// Package verify enforces the v4.8 Canonical Specification Structural Blueprint across Intent documents (Law 4 & Law 6).
package verify

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Canonical specification topology prefixes and anchor section titles.
const (
	rootManifestRelPath     = "intent.md"
	flatSpecsDirPrefix      = "architecture/specs/"
	canonicalSection1Title  = "Executive Summary & Problem Statement"
	canonicalSection2Title  = "Axiomatic Invariants & Governing Laws"
	canonicalVectorsTitle   = "Table-Driven Verification Vectors"
	canonicalClosingTitle   = "Cross-Plane Traceability & DoD Verification"
	markdownHorizontalRule  = "---"
	minCanonicalSpecH2Count = 5
	minCanonicalVectorRows  = 3
)

var (
	numberedH2HeadingPattern = regexp.MustCompile(`^## (\d+)\. (.+)$`)
	canonicalVectorIDPattern = regexp.MustCompile(`\bTC-[A-Z0-9]+-[0-9]{2,3}\b`)
	legacyVectorRowPattern   = regexp.MustCompile(`\|\s*` + "`?" + `(?:TV|VEC|CDV)-[A-Z0-9-]+` + "`?" + `\s*\|`)
	legacyFlatGateRefPattern = regexp.MustCompile(`\bGate\s+\d+\b`)
	danglingADRRefPattern    = regexp.MustCompile(`\bADR-\d+\b`)
	compiledGoSymbolPattern  = regexp.MustCompile(`\b(type|func|const|var)\s+[A-Za-z_]`)
)

// specBodyScan aggregates structural landmarks discovered outside fenced Markdown code blocks.
type specBodyScan struct {
	lines        []string
	h1Indices    []int
	h2Entries    []h2HeadingEntry
	hrIndices    map[int]bool
	goBlocks     []string
	proseIssue   string
}

type h2HeadingEntry struct {
	lineIdx int
	rawLine string
}

// validateStrictSpecTopology enforces flat intent/architecture/specs/*_spec.md placement with zero nested folders.
func validateStrictSpecTopology(rel string) string {
	if rel == rootManifestRelPath {
		return ""
	}
	if strings.Contains("/"+rel, forbiddenHandbookSegment) {
		return "handbook documents are prohibited in open-core intent/; relocate handbooks to sovereign docs (Law 6)"
	}
	if !strings.HasPrefix(rel, flatSpecsDirPrefix) {
		return fmt.Sprintf("unexpected Intent document path %q; expected %q or '%s*_spec.md' (Law 6)", rel, rootManifestRelPath, flatSpecsDirPrefix)
	}
	leaf := strings.TrimPrefix(rel, flatSpecsDirPrefix)
	if strings.Contains(leaf, "/") {
		return fmt.Sprintf("nested subdirectory under %s is prohibited (%q); flatten specifications directly into %s (Law 6)", flatSpecsDirPrefix, rel, flatSpecsDirPrefix)
	}
	if !strings.HasSuffix(leaf, "_spec.md") {
		return "specification under architecture/specs/ must use canonical '_spec.md' suffix (Law 6)"
	}
	return ""
}

// validateStrictSpecStructure enforces the v4.8 Canonical Specification Structural Blueprint on body text.
func validateStrictSpecStructure(body string) string {
	scan := scanSpecBodyOutsideFences(body)
	if scan.proseIssue != "" {
		return scan.proseIssue
	}
	if openIssue := validateCleanPostH1Opening(scan); openIssue != "" {
		return openIssue
	}
	if spineIssue := validateContiguousH2Spine(scan); spineIssue != "" {
		return spineIssue
	}
	if goIssue := validateCompiledGoContractBlocks(scan.goBlocks); goIssue != "" {
		return goIssue
	}
	return validateVerificationVectorsSection(scan)
}

// scanSpecBodyOutsideFences extracts headings, horizontal rules, Go blocks, and prose hygiene violations.
func scanSpecBodyOutsideFences(body string) specBodyScan {
	lines := strings.Split(body, "\n")
	scan := specBodyScan{lines: lines, hrIndices: make(map[int]bool)}
	inFence, inGo := false, false
	fenceChar, fenceLen := byte(0), 0
	var curGo []string

	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		ch, count := countLeadingFenceChars(trimmed)
		if !inFence && count >= 3 {
			inFence = true
			fenceChar, fenceLen = ch, count
			inGo = strings.EqualFold(strings.TrimSpace(trimmed[count:]), "go")
			curGo = nil
			continue
		}
		if inFence {
			if ch == fenceChar && count >= fenceLen && strings.TrimSpace(trimmed[count:]) == "" {
				if inGo {
					scan.goBlocks = append(scan.goBlocks, strings.Join(curGo, "\n"))
				}
				inFence, inGo, fenceChar, fenceLen = false, false, 0, 0
			} else if inGo {
				curGo = append(curGo, line)
			}
			continue
		}
		recordUnfencedSpecLine(&scan, idx, line, trimmed)
	}
	return scan
}

// recordUnfencedSpecLine records H1/H2/HR landmarks and checks legacy Gate/ADR prose tokens.
func recordUnfencedSpecLine(scan *specBodyScan, idx int, line, trimmed string) {
	if scan.proseIssue == "" {
		if legacyFlatGateRefPattern.MatchString(line) {
			scan.proseIssue = fmt.Sprintf("obsolete flat 'Gate <N>' citation on body line %d; use canonical 'Cell <Law>.<Ring> (<probe_id>)' coordinates", idx+1)
		} else if danglingADRRefPattern.MatchString(line) {
			scan.proseIssue = fmt.Sprintf("dangling 'ADR-XXXX' shorthand on body line %d; use self-contained architectural names", idx+1)
		}
	}
	if strings.HasPrefix(line, "# ") {
		scan.h1Indices = append(scan.h1Indices, idx)
	} else if strings.HasPrefix(line, "## ") {
		scan.h2Entries = append(scan.h2Entries, h2HeadingEntry{lineIdx: idx, rawLine: line})
	} else if trimmed == markdownHorizontalRule {
		scan.hrIndices[idx] = true
	}
}

// validateCleanPostH1Opening requires exactly one '# <Title>' followed solely by '---' before '## 1.'.
func validateCleanPostH1Opening(scan specBodyScan) string {
	if len(scan.h1Indices) != 1 {
		return fmt.Sprintf("expected exactly 1 top-level '# <Title>' heading outside code fences, found %d", len(scan.h1Indices))
	}
	h1Idx := scan.h1Indices[0]
	for i := 0; i < h1Idx; i++ {
		if strings.TrimSpace(scan.lines[i]) != "" {
			return "unexpected preamble content before '# <Title>' heading (Law 6)"
		}
	}
	if len(scan.h2Entries) == 0 {
		return "specification missing top-level '## <N>. <Title>' sections"
	}
	firstH2 := scan.h2Entries[0].lineIdx
	var between []string
	for i := h1Idx + 1; i < firstH2; i++ {
		if s := strings.TrimSpace(scan.lines[i]); s != "" {
			between = append(between, s)
		}
	}
	if len(between) != 1 || between[0] != markdownHorizontalRule {
		return "between '# <Title>' and '## 1.', expected strictly a single '---' divider with zero preamble prose or metadata blocks (Law 6)"
	}
	return ""
}

// validateContiguousH2Spine enforces 1..N (N >= 5) H2 numbering, '---' dividers, and canonical anchor titles.
func validateContiguousH2Spine(scan specBodyScan) string {
	total := len(scan.h2Entries)
	if total < minCanonicalSpecH2Count {
		return fmt.Sprintf("specification requires at least %d contiguous '## <N>.' sections, found %d", minCanonicalSpecH2Count, total)
	}
	for i, entry := range scan.h2Entries {
		expectedNum := i + 1
		m := numberedH2HeadingPattern.FindStringSubmatch(entry.rawLine)
		if len(m) != 3 {
			return fmt.Sprintf("unnumbered or malformed H2 heading %q; expected '## %d. <Title>' (Law 6)", entry.rawLine, expectedNum)
		}
		actualNum, _ := strconv.Atoi(m[1])
		title := strings.TrimSpace(m[2])
		if actualNum != expectedNum {
			return fmt.Sprintf("non-contiguous H2 section number %d in %q; expected %d (Law 6)", actualNum, entry.rawLine, expectedNum)
		}
		if titleErr := checkCanonicalAnchorTitle(expectedNum, total, title, entry.rawLine); titleErr != "" {
			return titleErr
		}
		if i > 0 && !hasHorizontalRuleBetween(scan.hrIndices, scan.h2Entries[i-1].lineIdx, entry.lineIdx) {
			return fmt.Sprintf("missing '---' horizontal rule divider preceding %q (Law 6)", entry.rawLine)
		}
	}
	return ""
}

// checkCanonicalAnchorTitle validates statutory H2 titles at sections 1, 2, N-1, and N.
func checkCanonicalAnchorTitle(num, total int, title, rawLine string) string {
	if num == 1 && title != canonicalSection1Title {
		return fmt.Sprintf("section 1 must be '## 1. %s', got %q", canonicalSection1Title, rawLine)
	}
	if num == 2 && title != canonicalSection2Title {
		return fmt.Sprintf("section 2 must be '## 2. %s', got %q", canonicalSection2Title, rawLine)
	}
	if num == total-1 && title != canonicalVectorsTitle {
		return fmt.Sprintf("penultimate section %d must be '## %d. %s', got %q (Law 2)", num, num, canonicalVectorsTitle, rawLine)
	}
	if num == total && title != canonicalClosingTitle {
		return fmt.Sprintf("closing section %d must be '## %d. %s', got %q (Law 4)", num, num, canonicalClosingTitle, rawLine)
	}
	return ""
}

// hasHorizontalRuleBetween checks whether at least one '---' line exists strictly between two line indices.
func hasHorizontalRuleBetween(hrIndices map[int]bool, startIdx, endIdx int) bool {
	for i := startIdx + 1; i < endIdx; i++ {
		if hrIndices[i] {
			return true
		}
	}
	return false
}

// validateCompiledGoContractBlocks requires at least one fenced Go block declaring compiled Go contracts.
func validateCompiledGoContractBlocks(goBlocks []string) string {
	if len(goBlocks) == 0 {
		return "specification missing required fenced '```go' SSoT contract block (Law 4)"
	}
	for _, block := range goBlocks {
		if compiledGoSymbolPattern.MatchString(block) {
			return ""
		}
	}
	return "fenced '```go' block(s) must declare at least one compiled Go type, func, const, or var contract (Law 4)"
}

// validateVerificationVectorsSection requires >= 3 canonical TC-<DOMAIN>-<NN> rows and zero legacy prefixes.
func validateVerificationVectorsSection(scan specBodyScan) string {
	total := len(scan.h2Entries)
	start := scan.h2Entries[total-2].lineIdx
	end := scan.h2Entries[total-1].lineIdx
	section := strings.Join(scan.lines[start:end], "\n")
	if legacyVectorRowPattern.MatchString(section) {
		return "verification vectors table uses legacy TV-/VEC-/CDV- prefix; standardize on 'TC-<DOMAIN>-<NN>' (Law 6)"
	}
	matches := canonicalVectorIDPattern.FindAllString(section, -1)
	if len(matches) < minCanonicalVectorRows {
		return fmt.Sprintf("section '%s' requires at least %d 'TC-<DOMAIN>-<NN>' vector IDs, found %d (Law 2)", canonicalVectorsTitle, minCanonicalVectorRows, len(matches))
	}
	return ""
}
