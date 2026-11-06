// Package verify provides deterministic Mermaid diagram syntax validation for Intent documents (Law 2 & Law 4).
package verify

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	htmlEntitySemicolonPattern = regexp.MustCompile(`&[A-Za-z0-9#]+;`)
	unquotedFlowNodeParenRegex = regexp.MustCompile(`\b[A-Za-z0-9_-]+\[[^"\(\]\n]*[()][^"\]\n]*\]`)
	unquotedFlowEdgeHazardRe   = regexp.MustCompile(`\|[^"|\n]*(?:[()]|/\*)[^"|\n]*\|`)
)

// validateMarkdownMermaidBlocks inspects all fenced ```mermaid blocks in a Markdown body for parser/orphan hazards.
func validateMarkdownMermaidBlocks(body string) string {
	blocks := extractMermaidCodeFences(body)
	for idx, block := range blocks {
		if issue := auditSingleMermaidBlock(idx+1, block); issue != "" {
			return issue
		}
	}
	return ""
}

// extractMermaidCodeFences extracts raw contents of all ```mermaid fenced code blocks.
func extractMermaidCodeFences(body string) []string {
	lines := strings.Split(body, "\n")
	var blocks []string
	var cur []string
	inFence, inMermaid := false, false
	fenceChar, fenceLen := byte(0), 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		ch, count := countLeadingFenceChars(trimmed)
		if !inFence && count >= 3 {
			inFence = true
			fenceChar, fenceLen = ch, count
			inMermaid = strings.EqualFold(strings.TrimSpace(trimmed[count:]), "mermaid")
			cur = nil
			continue
		}
		if inFence {
			if ch == fenceChar && count >= fenceLen && strings.TrimSpace(trimmed[count:]) == "" {
				if inMermaid {
					blocks = append(blocks, strings.Join(cur, "\n"))
				}
				inFence, inMermaid, fenceChar, fenceLen = false, false, 0, 0
			} else if inMermaid {
				cur = append(cur, line)
			}
		}
	}
	return blocks
}

// auditSingleMermaidBlock checks diagram-type-specific syntax hazards that break Mermaid v11 or spawn orphan nodes.
func auditSingleMermaidBlock(diagramNum int, block string) string {
	header := firstMermaidHeaderLine(block)
	if header == "" {
		return fmt.Sprintf("Mermaid diagram #%d is empty", diagramNum)
	}
	lowerHeader := strings.ToLower(header)
	isStateOrSeq := strings.HasPrefix(lowerHeader, "statediagram") || strings.HasPrefix(lowerHeader, "sequencediagram")
	isFlowchart := strings.HasPrefix(lowerHeader, "flowchart ") || strings.HasPrefix(lowerHeader, "graph ")

	for lineIdx, rawLine := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		if styleErr := checkMermaidStyleRgbaHazard(diagramNum, lineIdx+1, trimmed); styleErr != "" {
			return styleErr
		}
		if isStateOrSeq {
			if semiErr := checkStateOrSeqSemicolonHazard(diagramNum, lineIdx+1, trimmed); semiErr != "" {
				return semiErr
			}
		}
		if isFlowchart {
			if flowErr := checkFlowchartUnquotedHazard(diagramNum, lineIdx+1, trimmed); flowErr != "" {
				return flowErr
			}
		}
	}
	return ""
}

// firstMermaidHeaderLine returns the first non-empty, non-comment header line of a Mermaid block.
func firstMermaidHeaderLine(block string) string {
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "%%") {
			return trimmed
		}
	}
	return ""
}

// checkMermaidStyleRgbaHazard rejects comma-delimited rgba(...) values inside classDef or style declarations.
func checkMermaidStyleRgbaHazard(diagramNum, lineNum int, trimmed string) string {
	lower := strings.ToLower(trimmed)
	if (strings.HasPrefix(lower, "classdef ") || strings.HasPrefix(lower, "style ")) && strings.Contains(lower, "rgba(") {
		return fmt.Sprintf("Mermaid diagram #%d line %d uses 'rgba(...)' inside style/classDef; use hex color tokens without internal commas", diagramNum, lineNum)
	}
	return ""
}

// checkStateOrSeqSemicolonHazard rejects unescaped ';' statement terminators inside stateDiagram/sequenceDiagram lines.
func checkStateOrSeqSemicolonHazard(diagramNum, lineNum int, trimmed string) string {
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "classdef ") || strings.HasPrefix(lower, "style ") {
		return ""
	}
	scrubbed := htmlEntitySemicolonPattern.ReplaceAllString(trimmed, "")
	if strings.Contains(scrubbed, ";") {
		return fmt.Sprintf("Mermaid diagram #%d line %d contains unescaped ';' in stateDiagram/sequenceDiagram (terminates statement and spawns orphan nodes); use ',' or '&#59;'", diagramNum, lineNum)
	}
	return ""
}

// checkFlowchartUnquotedHazard rejects unquoted parentheses in [...] nodes or unquoted parentheses/'/*' in |...| edges.
func checkFlowchartUnquotedHazard(diagramNum, lineNum int, trimmed string) string {
	if unquotedFlowNodeParenRegex.MatchString(trimmed) {
		return fmt.Sprintf("Mermaid diagram #%d line %d has unquoted parentheses inside '[...]' node label; wrap label in double quotes e.g. Node[\"Label (Detail)\"]", diagramNum, lineNum)
	}
	if unquotedFlowEdgeHazardRe.MatchString(trimmed) {
		return fmt.Sprintf("Mermaid diagram #%d line %d has unquoted '(' / ')' or '/*' inside '|...|' edge label; wrap edge label in double quotes e.g. -->|\"Label (Detail)\"|", diagramNum, lineNum)
	}
	return ""
}
