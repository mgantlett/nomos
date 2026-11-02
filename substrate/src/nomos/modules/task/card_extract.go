// Package task defines domain models, schema constraints, and tracking mechanisms
// synthesized directly from authoritative Intent CUE schemas (task.cue).
package task

import "strings"

// ParsedTaskCard holds structured sections extracted from a 3-section Markdown task card.
type ParsedTaskCard struct {
	Scope              string
	AcceptanceCriteria []string
	Verification       string
}

// ExtractTaskCardSections parses a Markdown task description into structured Scope,
// Acceptance Criteria items (with checkbox markers stripped), and Verification text.
// @intent INT-874 #TaskCardInput
func ExtractTaskCardSections(body string) ParsedTaskCard {
	cleaned := strings.TrimSpace(htmlCommentRe.ReplaceAllString(body, ""))
	if cleaned == "" {
		return ParsedTaskCard{}
	}
	sections := parseCardSections(cleaned)
	return ParsedTaskCard{
		Scope:              cleanSectionBlock(sections["scope"]),
		AcceptanceCriteria: extractACLines(sections["ac"]),
		Verification:       cleanSectionBlock(sections["verify"]),
	}
}

func cleanSectionBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractACLines(lines []string) []string {
	var items []string
	for _, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if m := checkboxRe.FindStringSubmatch(trimmed); m != nil {
			if txt := strings.TrimSpace(m[2]); txt != "" {
				items = append(items, txt)
			}
			continue
		}
		trimmed = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* "))
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}
