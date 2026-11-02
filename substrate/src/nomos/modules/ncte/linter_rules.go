// Package ncte implements the Nomos Controlled Technical English (NCTE-100) lexicon and AST linter.
package ncte

import (
	"strings"
	"unicode"
)

// SplitSentences splits prose into NCTESentence records without splitting numeric decimals or version tags.
func SplitSentences(text string, lineNum int) []NCTESentence {
	var rawParts []string
	var buf strings.Builder
	runes := []rune(strings.TrimSpace(text))
	inBacktick := false

	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if ch == '`' {
			inBacktick = !inBacktick
		}
		buf.WriteRune(ch)
		if !inBacktick && (ch == '.' || ch == '?' || ch == '!') {
			if isDecimalOrVersionDot(runes, i) {
				continue
			}
			if i == len(runes)-1 || unicode.IsSpace(runes[i+1]) {
				part := strings.TrimSpace(buf.String())
				if part != "" {
					rawParts = append(rawParts, part)
				}
				buf.Reset()
			}
		}
	}
	if tail := strings.TrimSpace(buf.String()); tail != "" {
		rawParts = append(rawParts, tail)
	}

	out := make([]NCTESentence, 0, len(rawParts))
	for _, raw := range rawParts {
		toks := TokenizeSentence(raw, lineNum)
		if len(toks) == 0 {
			continue
		}
		proc := isProceduralSentence(toks)
		out = append(out, NCTESentence{
			RawText:      raw,
			WordCount:    len(toks),
			IsProcedural: proc,
			HasPassive:   detectPassiveVoice(toks, 2),
			LineNumber:   lineNum,
			Tokens:       toks,
		})
	}
	return out
}

// TokenizeSentence converts a sentence into NCTEToken items, collapsing each inline code span into 1 literal token.
func TokenizeSentence(raw string, lineNum int) []NCTEToken {
	var literals []string
	masked := inlineCodeRe.ReplaceAllStringFunc(raw, func(m string) string {
		literals = append(literals, m)
		return " __NCTE_CODE_LITERAL__ "
	})

	fields := strings.Fields(masked)
	tokens := make([]NCTEToken, 0, len(fields))
	litIdx := 0
	col := 1
	for _, f := range fields {
		if f == "__NCTE_CODE_LITERAL__" && litIdx < len(literals) {
			lit := literals[litIdx]
			litIdx++
			tokens = append(tokens, NCTEToken{Text: lit, Line: lineNum, Column: col, IsLiteral: true})
			col += len(lit) + 1
			continue
		}
		cleaned := wordCleanRe.ReplaceAllString(f, "")
		if cleaned == "" {
			continue
		}
		tokens = append(tokens, NCTEToken{Text: cleaned, Line: lineNum, Column: col})
		col += len(f) + 1
	}
	return tokens
}

func isDecimalOrVersionDot(runes []rune, idx int) bool {
	if runes[idx] != '.' || idx == 0 || idx+1 >= len(runes) {
		return false
	}
	return unicode.IsDigit(runes[idx-1]) && unicode.IsDigit(runes[idx+1])
}

func isProceduralSentence(tokens []NCTEToken) bool {
	if len(tokens) == 0 || tokens[0].IsLiteral {
		return false
	}
	first := strings.ToLower(tokens[0].Text)
	if first == "do" && len(tokens) > 1 && strings.ToLower(tokens[1].Text) == "not" {
		return true
	}
	return proceduralLeadVerbs[first]
}

func detectPassiveVoice(tokens []NCTEToken, maxGap int) bool {
	for i := 0; i < len(tokens); i++ {
		if tokens[i].IsLiteral || !beForms[strings.ToLower(tokens[i].Text)] {
			continue
		}
		limit := i + 1 + maxGap
		if limit >= len(tokens) {
			limit = len(tokens) - 1
		}
		for j := i + 1; j <= limit; j++ {
			if tokens[j].IsLiteral {
				break
			}
			w := strings.ToLower(tokens[j].Text)
			if isPastParticiple(w) {
				return true
			}
			if !strings.HasSuffix(w, "ly") && w != "not" && w != "never" && w != "always" {
				break
			}
		}
	}
	return false
}

func isPastParticiple(word string) bool {
	if irregularParticiples[word] {
		return true
	}
	return len(word) > 4 && strings.HasSuffix(word, "ed")
}

func (e *Engine) detectBannedModal(s NCTESentence) string {
	lowerProse := nonLiteralPhrase(s.Tokens)
	for _, modal := range e.Policy.BannedModals {
		m := strings.ToLower(modal)
		if strings.Contains(m, " ") || strings.Contains(m, "/") {
			if strings.Contains(lowerProse, m) {
				return modal
			}
			continue
		}
		for _, tok := range s.Tokens {
			if !tok.IsLiteral && strings.ToLower(tok.Text) == m {
				return modal
			}
		}
	}
	return ""
}

func detectCompoundProceduralAction(tokens []NCTEToken) bool {
	for i := 1; i+1 < len(tokens); i++ {
		if tokens[i].IsLiteral {
			continue
		}
		conj := strings.ToLower(tokens[i].Text)
		if (conj == "and" || conj == "then") && !tokens[i+1].IsLiteral {
			next := strings.ToLower(tokens[i+1].Text)
			if proceduralLeadVerbs[next] {
				return true
			}
		}
	}
	return false
}

func (e *Engine) detectBannedSynonym(s NCTESentence) (string, string) {
	lowerProse := " " + nonLiteralPhrase(s.Tokens) + " "
	for _, entry := range e.Policy.Vocabulary {
		for _, syn := range entry.BannedSynonyms {
			target := " " + strings.ToLower(syn) + " "
			if strings.Contains(lowerProse, target) {
				return syn, entry.CanonicalTerm
			}
		}
	}
	return "", ""
}

func detectSubjectiveAssertion(tokens []NCTEToken) string {
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i].IsLiteral || tokens[i+1].IsLiteral {
			continue
		}
		w1 := strings.ToLower(tokens[i].Text)
		w2 := strings.ToLower(tokens[i+1].Text)
		if subjectiveIntensifiers[w1] && subjectiveAdjectives[w2] {
			return tokens[i].Text + " " + tokens[i+1].Text
		}
	}
	return ""
}

func nonLiteralPhrase(tokens []NCTEToken) string {
	words := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if !t.IsLiteral {
			words = append(words, strings.ToLower(t.Text))
		}
	}
	return strings.Join(words, " ")
}

func skipYAMLFrontmatter(lines []string) int {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i + 1
		}
	}
	return 0
}

func isFenceDelimiter(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

func isListItem(trimmed string) bool {
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return true
	}
	for i, r := range trimmed {
		if unicode.IsDigit(r) {
			continue
		}
		return i > 0 && r == '.' && i+1 < len(trimmed) && trimmed[i+1] == ' '
	}
	return false
}

func stripListPrefix(trimmed string) string {
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return strings.TrimSpace(trimmed[2:])
	}
	if idx := strings.Index(trimmed, ". "); idx > 0 {
		return strings.TrimSpace(trimmed[idx+2:])
	}
	return trimmed
}

func splitMarkdownRow(trimmed string) []string {
	raw := strings.Split(strings.Trim(trimmed, "|"), "|")
	cells := make([]string, 0, len(raw))
	for _, c := range raw {
		cells = append(cells, strings.TrimSpace(c))
	}
	return cells
}

func isTableSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		clean := strings.Trim(c, ":- ")
		if clean != "" {
			return false
		}
	}
	return true
}

func (e *Engine) isExemptHeader(header string) bool {
	clean := strings.ToLower(strings.Trim(header, "*` "))
	for _, ex := range e.Policy.ExemptTableHeaders {
		if clean == strings.ToLower(ex) {
			return true
		}
	}
	return false
}

func isLiteralOrQuotedCell(cell string) bool {
	if cell == "" {
		return true
	}
	if (strings.HasPrefix(cell, "`") && strings.HasSuffix(cell, "`")) ||
		(strings.HasPrefix(cell, "\"") && strings.HasSuffix(cell, "\"")) {
		return true
	}
	return false
}
