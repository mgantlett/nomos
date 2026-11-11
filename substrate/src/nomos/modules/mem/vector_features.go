// Package mem implements the Nomos Mneme receipt-gated episodic memory system.
package mem

import (
	"crypto/sha256"
	"strconv"
	"strings"
	"unicode"
)

const (
	wordFeatureWeight    float32 = 1.0
	stemFeatureWeight    float32 = 0.85
	trigramFeatureWeight float32 = 0.35
	minSubwordTokenLen           = 4
	subwordGramSize              = 4
)

type weightedFeature struct {
	term   string
	weight float32
}

var commonStopWords = map[string]bool{
	"about": true, "above": true, "across": true, "after": true, "again": true,
	"against": true, "all": true, "also": true, "an": true, "and": true,
	"any": true, "are": true, "as": true, "at": true, "be": true,
	"because": true, "been": true, "before": true, "being": true, "below": true,
	"between": true, "both": true, "but": true, "by": true, "can": true,
	"did": true, "do": true, "does": true, "doing": true, "down": true,
	"during": true, "each": true, "few": true, "for": true, "from": true,
	"further": true, "had": true, "has": true, "have": true, "having": true,
	"here": true, "how": true, "if": true, "in": true, "into": true,
	"is": true, "it": true, "its": true, "just": true, "more": true,
	"most": true, "no": true, "nor": true, "not": true, "now": true,
	"of": true, "off": true, "on": true, "once": true, "only": true,
	"or": true, "other": true, "our": true, "out": true, "over": true,
	"own": true, "per": true, "same": true, "so": true, "some": true,
	"such": true, "than": true, "that": true, "the": true, "their": true,
	"them": true, "then": true, "there": true, "these": true, "they": true,
	"this": true, "those": true, "through": true, "to": true, "too": true,
	"under": true, "until": true, "up": true, "very": true, "via": true,
	"was": true, "we": true, "were": true, "what": true, "when": true,
	"where": true, "which": true, "while": true, "who": true, "whom": true,
	"why": true, "will": true, "with": true, "without": true, "you": true,
	"your": true,
}

// tokenizeWords splits identifiers and prose into stemmed lowercase content tokens with stop-word removal.
func tokenizeWords(s string) []string {
	raw := splitIdentifierTokens(s)
	if len(raw) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(raw))
	for _, tok := range raw {
		if !commonStopWords[tok] {
			filtered = append(filtered, stemToken(tok))
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	return raw
}

// splitIdentifierTokens splits camelCase, PascalCase, snake_case, and path delimiters into lowercase words.
func splitIdentifierTokens(s string) []string {
	chunks := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	var tokens []string
	for _, chunk := range chunks {
		parts := splitCamelChunk(chunk)
		for _, part := range parts {
			low := strings.ToLower(part)
			if len(low) > 1 {
				tokens = append(tokens, low)
			}
		}
	}
	return tokens
}

// splitCamelChunk decomposes a contiguous alphanumeric token across case and digit boundaries.
func splitCamelChunk(chunk string) []string {
	runes := []rune(chunk)
	if len(runes) <= 1 {
		return nil
	}
	var parts []string
	start := 0
	for i := 1; i < len(runes); i++ {
		if isCamelBoundary(runes, i) {
			parts = append(parts, string(runes[start:i]))
			start = i
		}
	}
	parts = append(parts, string(runes[start:]))
	return parts
}

func isCamelBoundary(runes []rune, idx int) bool {
	prev := runes[idx-1]
	curr := runes[idx]
	if unicode.IsLower(prev) && unicode.IsUpper(curr) {
		return true
	}
	if unicode.IsDigit(prev) != unicode.IsDigit(curr) {
		return true
	}
	if idx+1 < len(runes) && unicode.IsUpper(prev) && unicode.IsUpper(curr) && unicode.IsLower(runes[idx+1]) {
		return true
	}
	return false
}

// stemToken reduces common English and code identifier inflections to a canonical stem.
func stemToken(w string) string {
	w = stripPluralSuffix(w)
	w = stripInflectionSuffix(w)
	return collapseTrailingDoubleConsonant(w)
}

func stripPluralSuffix(w string) string {
	if len(w) > 4 && strings.HasSuffix(w, "ies") {
		return w[:len(w)-3] + "y"
	}
	if len(w) > 4 && hasSibilantPlural(w) {
		return w[:len(w)-2]
	}
	if len(w) > 3 && strings.HasSuffix(w, "s") && !hasProtectedSingularS(w) {
		return w[:len(w)-1]
	}
	return w
}

func hasSibilantPlural(w string) bool {
	for _, suf := range []string{"ches", "shes", "xes", "zes", "ses"} {
		if strings.HasSuffix(w, suf) {
			return true
		}
	}
	return false
}

func hasProtectedSingularS(w string) bool {
	for _, suf := range []string{"ss", "us", "is", "os"} {
		if strings.HasSuffix(w, suf) {
			return true
		}
	}
	return false
}

func stripInflectionSuffix(w string) string {
	for _, suf := range []string{"ing", "ion"} {
		if len(w) >= 6 && strings.HasSuffix(w, suf) {
			return w[:len(w)-3]
		}
	}
	if len(w) >= 6 && strings.HasSuffix(w, "ed") {
		return w[:len(w)-2]
	}
	if len(w) >= 5 && strings.HasSuffix(w, "e") && !strings.HasSuffix(w, "ee") {
		return w[:len(w)-1]
	}
	return w
}

func collapseTrailingDoubleConsonant(w string) string {
	n := len(w)
	if n < 4 || w[n-1] != w[n-2] {
		return w
	}
	last := w[n-1]
	if strings.ContainsRune("aeioulsz", rune(last)) {
		return w
	}
	return w[:n-1]
}

// extractSubwordFeatures builds weighted word, stem, and boundary-padded character 4-gram features.
func extractSubwordFeatures(text string) []weightedFeature {
	raw := splitIdentifierTokens(text)
	if len(raw) == 0 {
		return nil
	}
	features := make([]weightedFeature, 0, len(raw)*5)
	for _, tok := range raw {
		if commonStopWords[tok] {
			continue
		}
		stem := stemToken(tok)
		features = append(features,
			weightedFeature{term: "w:" + tok, weight: wordFeatureWeight},
			weightedFeature{term: "s:" + stem, weight: stemFeatureWeight},
		)
		if len(stem) >= minSubwordTokenLen {
			padded := "#" + stem + "#"
			for i := 0; i+subwordGramSize <= len(padded); i++ {
				features = append(features, weightedFeature{
					term:   "g:" + padded[i:i+subwordGramSize],
					weight: trigramFeatureWeight,
				})
			}
		}
	}
	return features
}

// projectFeatureDigest projects a weighted feature across dim independent zero-mean coordinates.
func projectFeatureDigest(vec []float32, feat weightedFeature) {
	dim := len(vec)
	blocks := (dim + sha256.Size - 1) / sha256.Size
	coord := 0
	for bIdx := 0; bIdx < blocks && coord < dim; bIdx++ {
		seed := strconv.Itoa(bIdx) + ":" + feat.term
		digest := sha256.Sum256([]byte(seed))
		for j := 0; j < sha256.Size && coord < dim; j++ {
			centered := (float32(digest[j]) - 127.5) / 127.5
			vec[coord] += centered * feat.weight
			coord++
		}
	}
}
