// Package capbac provides shared stateless POSIX shell quote tracking, segment splitting,
// and quoted word tokenization for container and host security airlocks.
// @intent nomos_hermetic_worktree_spec.md
package capbac

import (
	"path/filepath"
	"strings"
)

type shellQuoteState struct {
	inSingle bool
	inDouble bool
	escaped  bool
}

func (q *shellQuoteState) step(ch byte) (toggled, wasEscaped bool) {
	if q.escaped {
		q.escaped = false
		return false, true
	}
	if ch == '\\' && !q.inSingle {
		q.escaped = true
		return false, false
	}
	if ch == '\'' && !q.inDouble {
		q.inSingle = !q.inSingle
		return true, false
	}
	if ch == '"' && !q.inSingle {
		q.inDouble = !q.inDouble
		return true, false
	}
	return false, false
}

// SplitUnquotedShellSegments splits a compound shell command on unquoted ';', '\n', '|', '||', and '&&'.
// @intent nomos_hermetic_worktree_spec.md
func SplitUnquotedShellSegments(cmdLine string) []string {
	var segs []string
	var cur strings.Builder
	var qs shellQuoteState
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			segs = append(segs, s)
		}
		cur.Reset()
	}
	for i := 0; i < len(cmdLine); i++ {
		ch := cmdLine[i]
		toggled, wasEsc := qs.step(ch)
		if !wasEsc && !toggled && !qs.inSingle && !qs.inDouble {
			if adv, split := unquotedSeparatorStep(cmdLine, i); split {
				flush()
				i += adv
				continue
			}
		}
		cur.WriteByte(ch)
	}
	flush()
	return segs
}

func unquotedSeparatorStep(cmdLine string, idx int) (int, bool) {
	ch := cmdLine[idx]
	if ch == ';' || ch == '\n' {
		return 0, true
	}
	if ch == '|' {
		if idx+1 < len(cmdLine) && cmdLine[idx+1] == '|' {
			return 1, true
		}
		return 0, true
	}
	if ch == '&' && idx+1 < len(cmdLine) && cmdLine[idx+1] == '&' {
		return 1, true
	}
	return 0, false
}

// TokenizeQuotedShellWords tokenizes a shell segment into words while stripping outer quotes and escapes.
// @intent nomos_hermetic_worktree_spec.md
func TokenizeQuotedShellWords(seg string) []string {
	var out []string
	var cur strings.Builder
	var qs shellQuoteState
	haveTok := false
	flush := func() {
		if haveTok || cur.Len() > 0 {
			out = append(out, cur.String())
		}
		cur.Reset()
		haveTok = false
	}
	for i := 0; i < len(seg); i++ {
		ch := seg[i]
		toggled, wasEsc := qs.step(ch)
		if (!wasEsc && qs.escaped) || toggled {
			haveTok = true
			continue
		}
		if !wasEsc && !qs.inSingle && !qs.inDouble && (ch == ' ' || ch == '\t') {
			flush()
			continue
		}
		cur.WriteByte(ch)
		haveTok = true
	}
	flush()
	return out
}

// HasUnquotedShellChainingOrSub reports whether cmd contains unquoted shell separators,
// command/process substitution, ANSI-C escape quoting, or brace expansion.
// @intent nomos_hermetic_worktree_spec.md
func HasUnquotedShellChainingOrSub(cmd string) bool {
	if hasUnquotedBraceExpansion(cmd) {
		return true
	}
	var qs shellQuoteState
	for i := 0; i < len(cmd); i++ {
		toggled, wasEsc := qs.step(cmd[i])
		if wasEsc || toggled || qs.inSingle {
			continue
		}
		if isCmdSubstAt(cmd, i) || (!qs.inDouble && isUnquotedChainOrDialectAt(cmd, i)) {
			return true
		}
	}
	return false
}

func isCmdSubstAt(cmd string, i int) bool {
	return cmd[i] == '`' || (cmd[i] == '$' && i+1 < len(cmd) && cmd[i+1] == '(')
}

var unquotedChainSepBytes = [256]bool{
	';': true, '|': true, '&': true, '\n': true, '\r': true,
}

func isUnquotedChainOrDialectAt(cmd string, i int) bool {
	ch := cmd[i]
	if unquotedChainSepBytes[ch] {
		return true
	}
	if i+1 >= len(cmd) {
		return false
	}
	next := cmd[i+1]
	return ((ch == '<' || ch == '>') && next == '(') || (ch == '$' && next == '\'')
}

func hasUnquotedBraceExpansion(cmd string) bool {
	var qs shellQuoteState
	depth := 0
	commaAtDepth := false
	for i := 0; i < len(cmd); i++ {
		toggled, wasEsc := qs.step(cmd[i])
		if wasEsc || toggled || qs.inSingle || qs.inDouble {
			continue
		}
		var prev byte
		if i > 0 {
			prev = cmd[i-1]
		}
		var matched bool
		depth, commaAtDepth, matched = stepBraceExpansion(cmd[i], prev, depth, commaAtDepth)
		if matched {
			return true
		}
	}
	return false
}

func stepBraceExpansion(ch, prev byte, depth int, hasComma bool) (int, bool, bool) {
	if ch == '{' && prev != '$' {
		return depth + 1, hasComma, false
	}
	if depth == 0 {
		return 0, false, false
	}
	if ch == ',' {
		return depth, true, false
	}
	if ch == '}' {
		return depth - 1, false, hasComma
	}
	return depth, hasComma, false
}

var (
	shellWrapperBins       = map[string]bool{"env": true, "command": true, "nohup": true, "nice": true, "sudo": true, "exec": true, "timeout": true}
	shellWrapperValueFlags = map[string]bool{"-u": true, "--unset": true, "-C": true, "--chdir": true, "-S": true, "-n": true, "-k": true, "-s": true}
	gitGlobalValueFlags    = map[string]bool{"-C": true, "-c": true, "--git-dir": true, "--work-tree": true, "--namespace": true, "--exec-path": true, "--config-env": true}
)

func isPrivilegedReleaseCommand(cmd string) bool {
	for _, seg := range SplitUnquotedShellSegments(cmd) {
		words := stripShellCommandWrappers(TokenizeQuotedShellWords(seg))
		if len(words) < 2 {
			continue
		}
		if matchesReleaseSinkTokens(words) {
			return true
		}
	}
	return false
}

func stripShellCommandWrappers(words []string) []string {
	idx := 0
	for idx < len(words) {
		w := words[idx]
		if isEnvAssignmentToken(w) {
			idx++
			continue
		}
		base := normalizeExecName(w)
		if !shellWrapperBins[base] {
			break
		}
		idx++
		idx = skipWrapperArgs(base, words, idx)
	}
	return words[idx:]
}

func skipWrapperArgs(wrapper string, words []string, idx int) int {
	for idx < len(words) {
		w := words[idx]
		if w == "--" {
			return idx + 1
		}
		if wrapper == "env" && isEnvAssignmentToken(w) {
			idx++
			continue
		}
		if strings.HasPrefix(w, "-") {
			idx++
			if shellWrapperValueFlags[w] && idx < len(words) {
				idx++
			}
			continue
		}
		if wrapper == "timeout" && len(w) > 0 && w[0] >= '0' && w[0] <= '9' {
			idx++
			continue
		}
		break
	}
	return idx
}

func isEnvAssignmentToken(w string) bool {
	eq := strings.IndexByte(w, '=')
	return eq > 0 && !strings.HasPrefix(w, "-")
}

func normalizeExecName(tok string) string {
	base := strings.ToLower(filepath.Base(strings.ReplaceAll(tok, "\\", "/")))
	return strings.TrimSuffix(base, ".exe")
}

func matchesReleaseSinkTokens(words []string) bool {
	switch normalizeExecName(words[0]) {
	case "git":
		return firstSubcommand(words[1:], gitGlobalValueFlags) == "push"
	case "gh":
		return isGHReleaseOrMerge(words[1:])
	case "npm", "pnpm", "yarn", "cargo":
		return firstSubcommand(words[1:], nil) == "publish"
	case "twine":
		return firstSubcommand(words[1:], nil) == "upload"
	case "docker", "podman":
		return firstSubcommand(words[1:], nil) == "push"
	default:
		return false
	}
}

func firstSubcommand(args []string, valueFlags map[string]bool) string {
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if tok == "--" {
			return ""
		}
		if strings.HasPrefix(tok, "-") {
			if valueFlags != nil && valueFlags[tok] && i+1 < len(args) {
				i++
			}
			continue
		}
		return strings.ToLower(tok)
	}
	return ""
}

func isGHReleaseOrMerge(args []string) bool {
	sub := ""
	for _, tok := range args {
		if strings.HasPrefix(tok, "-") {
			continue
		}
		low := strings.ToLower(tok)
		if sub == "" {
			if low == "release" {
				return true
			}
			sub = low
			continue
		}
		return sub == "pr" && low == "merge"
	}
	return false
}
