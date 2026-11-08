// Package treesitter provides shared nested-function masking and multi-line signature joining.
package treesitter

import "strings"

type funcSpan struct {
	name      string
	startIdx  int
	headerEnd int
	endIdx    int
}

func maskNestedLines(lines []string, outerStart, outerEnd int, spans []funcSpan) []string {
	if outerStart > outerEnd || outerStart >= len(lines) {
		return nil
	}
	if outerEnd >= len(lines) {
		outerEnd = len(lines) - 1
	}
	out := make([]string, outerEnd-outerStart+1)
	copy(out, lines[outerStart:outerEnd+1])
	for _, sp := range spans {
		if sp.startIdx <= outerStart || sp.startIdx > outerEnd {
			continue
		}
		innerEnd := sp.endIdx
		if innerEnd > outerEnd {
			innerEnd = outerEnd
		}
		for idx := sp.startIdx; idx <= innerEnd; idx++ {
			out[idx-outerStart] = ""
		}
	}
	return out
}

func collapseHeaderLines(cleanLines []string, startIdx int) (string, int) {
	if startIdx < 0 || startIdx >= len(cleanLines) {
		return "", startIdx
	}
	first := cleanLines[startIdx]
	if isCommentOrBlankHeaderLine(strings.TrimSpace(first)) || !strings.Contains(first, "(") {
		return first, startIdx
	}
	return collectWrappedHeaderSpan(cleanLines, startIdx, 12)
}

func isCommentOrBlankHeaderLine(trimmed string) bool {
	return trimmed == "" || strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")
}

func collectWrappedHeaderSpan(cleanLines []string, startIdx, maxJoin int) (string, int) {
	var b strings.Builder
	parenBalance := 0
	limit := startIdx + maxJoin
	if limit > len(cleanLines) {
		limit = len(cleanLines)
	}
	for i := startIdx; i < limit; i++ {
		if i > startIdx {
			b.WriteByte(' ')
		}
		b.WriteString(cleanLines[i])
		parenBalance += strings.Count(cleanLines[i], "(") - strings.Count(cleanLines[i], ")")
		if parenBalance <= 0 && hasHeaderTerminator(cleanLines[i], i+1, limit, cleanLines) {
			return b.String(), i
		}
	}
	return cleanLines[startIdx], startIdx
}

func hasHeaderTerminator(line string, nextIdx, limit int, cleanLines []string) bool {
	if strings.Contains(line, "{") || strings.Contains(line, ":") || strings.Contains(line, "=>") {
		return true
	}
	return nextIdx < limit && strings.HasPrefix(strings.TrimSpace(cleanLines[nextIdx]), "{")
}
