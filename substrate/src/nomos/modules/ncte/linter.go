// Package ncte implements the Nomos Controlled Technical English (NCTE-100) lexicon and AST linter.
package ncte

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	inlineCodeRe = regexp.MustCompile("`[^`]+`")
	wordCleanRe  = regexp.MustCompile(`^[^a-zA-Z0-9]+|[^a-zA-Z0-9]+$`)

	beForms = map[string]bool{
		"am": true, "is": true, "are": true, "was": true,
		"were": true, "been": true, "being": true, "be": true,
	}
	irregularParticiples = map[string]bool{
		"built": true, "chosen": true, "done": true, "driven": true, "eaten": true,
		"fallen": true, "forbidden": true, "forgotten": true, "frozen": true,
		"given": true, "gone": true, "grown": true, "hidden": true, "held": true,
		"kept": true, "known": true, "left": true, "lost": true, "made": true,
		"meant": true, "met": true, "paid": true, "put": true, "read": true,
		"run": true, "said": true, "seen": true, "sent": true, "set": true,
		"shown": true, "shut": true, "sold": true, "spent": true, "spoken": true,
		"split": true, "spread": true, "stood": true, "struck": true, "taken": true,
		"taught": true, "thought": true, "thrown": true, "told": true,
		"understood": true, "worn": true, "written": true,
	}
	proceduralLeadVerbs = map[string]bool{
		"execute": true, "run": true, "start": true, "compile": true, "verify": true,
		"reject": true, "mutate": true, "prune": true, "seal": true, "pair": true,
		"commit": true, "review": true, "sync": true, "abort": true, "save": true,
		"recall": true, "distill": true, "handshake": true, "intercept": true,
		"attest": true, "ensure": true, "please": true, "declare": true, "state": true,
		"define": true, "write": true, "use": true, "prohibit": true, "restrict": true,
		"limit": true, "express": true, "create": true, "build": true, "update": true,
	}
	subjectiveIntensifiers = map[string]bool{
		"very": true, "extremely": true, "super": true, "remarkably": true,
		"incredibly": true, "fairly": true, "pretty": true, "quite": true,
	}
	subjectiveAdjectives = map[string]bool{
		"fast": true, "slow": true, "robust": true, "good": true, "bad": true,
		"nice": true, "clean": true, "easy": true, "simple": true, "modern": true,
		"flexible": true, "seamless": true, "powerful": true,
	}
)

// Engine implements deterministic NCTE-100 Markdown and sentence verification (#NCTEPolicy).
type Engine struct {
	Policy NCTEPolicy
}

// NewEngine constructs a deterministic NCTE-100 linter engine using DefaultPolicy when nil.
func NewEngine(policy *NCTEPolicy) *Engine {
	if policy == nil {
		p := DefaultPolicy()
		return &Engine{Policy: p}
	}
	return &Engine{Policy: *policy}
}

// LintMarkdown analyzes Markdown prose while isolating YAML frontmatter, fenced code, and inline literals.
func (e *Engine) LintMarkdown(filePath string, content []byte) (*NCTESpecReport, error) {
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	startIdx := skipYAMLFrontmatter(lines)

	rep := &NCTESpecReport{
		FilePath:   filePath,
		VerifiedAt: time.Now().UTC(),
	}
	inFence := false
	var paraLines []indexedLine
	var tableHeaders []string

	flushParagraph := func() {
		if len(paraLines) == 0 {
			return
		}
		e.lintParagraph(paraLines, rep)
		paraLines = nil
	}

	for i := startIdx; i < len(lines); i++ {
		raw := lines[i]
		trimmed := strings.TrimSpace(raw)
		if isFenceDelimiter(trimmed) {
			flushParagraph()
			tableHeaders = nil
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "---") {
			flushParagraph()
			tableHeaders = nil
			continue
		}
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			flushParagraph()
			tableHeaders = e.lintTableRow(trimmed, i+1, tableHeaders, rep)
			continue
		}
		tableHeaders = nil
		if isListItem(trimmed) {
			flushParagraph()
			e.lintSingleProseLine(stripListPrefix(trimmed), i+1, true, rep)
			continue
		}
		paraLines = append(paraLines, indexedLine{line: i + 1, text: trimmed})
	}
	flushParagraph()

	rep.ViolationsCount = len(rep.Diagnostics)
	rep.Passed = rep.ViolationsCount == 0
	return rep, nil
}

type indexedLine struct {
	line int
	text string
}

func (e *Engine) lintParagraph(lines []indexedLine, rep *NCTESpecReport) {
	joined := make([]string, 0, len(lines))
	for _, l := range lines {
		joined = append(joined, l.text)
	}
	sentences := SplitSentences(strings.Join(joined, " "), lines[0].line)
	if len(sentences) > e.Policy.Bounds.MaxParagraphSentences {
		rep.Diagnostics = append(rep.Diagnostics, NCTEDiagnostic{
			Code: ErrParagraphTooLong, Severity: e.Policy.Severity, Line: lines[0].line, Column: 1,
			Message:  fmt.Sprintf("paragraph contains %d sentences (max %d)", len(sentences), e.Policy.Bounds.MaxParagraphSentences),
			Sentence: sentences[0].RawText,
			Remedy:   "split paragraph into atomic sections of 6 sentences or fewer",
		})
	}
	for _, s := range sentences {
		rep.TotalSentences++
		rep.TotalWords += s.WordCount
		rep.Diagnostics = append(rep.Diagnostics, e.ValidateSentence(s)...)
	}
}

func (e *Engine) lintSingleProseLine(text string, lineNum int, forceProcedural bool, rep *NCTESpecReport) {
	for _, s := range SplitSentences(text, lineNum) {
		if forceProcedural && s.IsProcedural {
			s.IsProcedural = true
		}
		rep.TotalSentences++
		rep.TotalWords += s.WordCount
		rep.Diagnostics = append(rep.Diagnostics, e.ValidateSentence(s)...)
	}
}

func (e *Engine) lintTableRow(trimmed string, lineNum int, headers []string, rep *NCTESpecReport) []string {
	cells := splitMarkdownRow(trimmed)
	if isTableSeparatorRow(cells) {
		return headers
	}
	if len(headers) == 0 {
		return cells
	}
	for idx, cell := range cells {
		if idx < len(headers) && e.isExemptHeader(headers[idx]) {
			continue
		}
		if isLiteralOrQuotedCell(cell) {
			continue
		}
		toks := TokenizeSentence(cell, lineNum)
		if len(toks) > e.Policy.Bounds.MaxTableCellWords {
			rep.Diagnostics = append(rep.Diagnostics, NCTEDiagnostic{
				Code: ErrTableCellTooLong, Severity: e.Policy.Severity, Line: lineNum, Column: 1,
				Message:  fmt.Sprintf("table cell contains %d words (max %d)", len(toks), e.Policy.Bounds.MaxTableCellWords),
				Sentence: cell,
				Remedy:   "condense prose table cell to 12 words or fewer",
			})
		}
	}
	return headers
}

// ValidateSentence evaluates a single NCTESentence against all 5 NCTE-100 axiomatic rules.
func (e *Engine) ValidateSentence(sentence NCTESentence) []NCTEDiagnostic {
	var diags []NCTEDiagnostic
	maxWords := e.Policy.Bounds.MaxDescriptiveWords
	if sentence.IsProcedural {
		maxWords = e.Policy.Bounds.MaxProceduralWords
	}
	if sentence.WordCount > maxWords {
		diags = append(diags, e.diag(ErrSentenceTooLong, sentence,
			fmt.Sprintf("sentence length %d exceeds max %d words", sentence.WordCount, maxWords),
			fmt.Sprintf("rewrite sentence in %d words or fewer", maxWords)))
	}
	if detectPassiveVoice(sentence.Tokens, e.Policy.Bounds.MaxPassiveAdverbGap) {
		diags = append(diags, e.diag(ErrPassiveVoiceDetected, sentence,
			"passive voice construction detected", "rewrite in active voice with an explicit subject or imperative verb"))
	}
	if modal := e.detectBannedModal(sentence); modal != "" {
		diags = append(diags, e.diag(ErrAmbiguousModal, sentence,
			fmt.Sprintf("prohibited ambiguous modal or connector %q detected", modal),
			"replace modal phrasing with deterministic imperative or declarative bounds"))
	}
	if sentence.IsProcedural && detectCompoundProceduralAction(sentence.Tokens) {
		diags = append(diags, e.diag(ErrCompoundAction, sentence,
			"compound procedural instruction detected", "split conjunction-joined actions into discrete procedural steps"))
	}
	if syn, canon := e.detectBannedSynonym(sentence); syn != "" {
		diags = append(diags, e.diag(ErrNonSingularTerm, sentence,
			fmt.Sprintf("non-singular synonym %q violates Law 6", syn),
			fmt.Sprintf("replace %q with canonical NCTE-100 term %q", syn, canon)))
	}
	if phrase := detectSubjectiveAssertion(sentence.Tokens); phrase != "" {
		diags = append(diags, e.diag(ErrSubjectiveAssertion, sentence,
			fmt.Sprintf("subjective qualitative assertion %q lacks numeric bounds", phrase),
			"replace qualitative intensifiers with measurable latency, byte, or exit-code bounds"))
	}
	return diags
}

func (e *Engine) diag(code ErrorCode, s NCTESentence, msg, remedy string) NCTEDiagnostic {
	return NCTEDiagnostic{
		Code: code, Severity: e.Policy.Severity, Line: s.LineNumber, Column: 1,
		Message: msg, Sentence: s.RawText, Remedy: remedy,
	}
}
