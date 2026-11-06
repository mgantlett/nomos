// Package session implements polymorphic Tier 1 transcript discovery, Sentinel
// governance scoring, and RLVR/DPO dataset synthesis.
package session

import (
	"path/filepath"
	"strings"
)

type sentinelShellScanner struct {
	segments   [][]string
	curSeg     []string
	tok        strings.Builder
	tokStarted bool
	inSingle   bool
	inDouble   bool
}

// hasUnscopedGitCommitInvocation inspects unquoted shell command segments to detect
// direct 'git commit' executions outside an isolated 'worktrees/' directory.
func hasUnscopedGitCommitInvocation(cmdLine, initialCwd string) bool {
	if strings.TrimSpace(cmdLine) == "" {
		return false
	}
	segments := scanSentinelShellSegments(cmdLine)
	cwd := filepath.ToSlash(strings.TrimSpace(initialCwd))
	for _, seg := range segments {
		core := stripSentinelLeadingWrappers(seg)
		if len(core) == 0 {
			continue
		}
		exe := filepath.Base(filepath.ToSlash(core[0]))
		if exe == "cd" {
			cwd = resolveSentinelCdTarget(cwd, core[1:])
			continue
		}
		if exe != "git" {
			continue
		}
		subcmd, gitDir := parseGitSubcommandAndDir(core[1:], cwd)
		if subcmd == "commit" && !isWorktreeDirectoryPath(gitDir) {
			return true
		}
	}
	return false
}

func isWorktreeDirectoryPath(dir string) bool {
	norm := filepath.ToSlash(strings.TrimSpace(dir))
	return strings.Contains(norm, "worktrees/")
}

func scanSentinelShellSegments(cmdLine string) [][]string {
	var sc sentinelShellScanner
	for i := 0; i < len(cmdLine); i++ {
		i = sc.step(cmdLine, i)
	}
	sc.flushSegment()
	return sc.segments
}

func (sc *sentinelShellScanner) step(s string, i int) int {
	ch := s[i]
	if sc.inSingle {
		if ch == '\'' {
			sc.inSingle = false
		} else {
			sc.tok.WriteByte(ch)
		}
		return i
	}
	if sc.inDouble {
		if ch == '\\' && i+1 < len(s) {
			sc.tok.WriteByte(s[i+1])
			return i + 1
		}
		if ch == '"' {
			sc.inDouble = false
		} else {
			sc.tok.WriteByte(ch)
		}
		return i
	}
	return sc.stepUnquoted(s, i)
}

func (sc *sentinelShellScanner) stepUnquoted(s string, i int) int {
	ch := s[i]
	if ch == '\\' && i+1 < len(s) {
		sc.tokStarted = true
		sc.tok.WriteByte(s[i+1])
		return i + 1
	}
	if ch == '\'' || ch == '"' {
		sc.tokStarted = true
		sc.inSingle = ch == '\''
		sc.inDouble = ch == '"'
		return i
	}
	if ch == ';' || ch == '|' || ch == '&' || ch == '\n' || ch == '(' || ch == ')' || ch == '`' {
		sc.flushSegment()
		return i
	}
	if ch == ' ' || ch == '\t' || ch == '\r' {
		sc.flushToken()
		return i
	}
	sc.tokStarted = true
	sc.tok.WriteByte(ch)
	return i
}

func (sc *sentinelShellScanner) flushToken() {
	if sc.tokStarted {
		sc.curSeg = append(sc.curSeg, sc.tok.String())
		sc.tok.Reset()
		sc.tokStarted = false
	}
}

func (sc *sentinelShellScanner) flushSegment() {
	sc.flushToken()
	if len(sc.curSeg) > 0 {
		sc.segments = append(sc.segments, sc.curSeg)
		sc.curSeg = nil
	}
}

func stripSentinelLeadingWrappers(tokens []string) []string {
	idx := 0
	for idx < len(tokens) {
		tok := tokens[idx]
		if isEnvAssignmentToken(tok) {
			idx++
			continue
		}
		base := filepath.Base(filepath.ToSlash(tok))
		if base == "env" || base == "sudo" || base == "command" {
			idx++
			continue
		}
		break
	}
	return tokens[idx:]
}

func isEnvAssignmentToken(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	for i := 0; i < eq; i++ {
		c := tok[i]
		valid := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || (i > 0 && c >= '0' && c <= '9')
		if !valid {
			return false
		}
	}
	return true
}

func resolveSentinelCdTarget(currentCwd string, args []string) string {
	for len(args) > 0 && (args[0] == "-L" || args[0] == "-P" || args[0] == "--") {
		args = args[1:]
	}
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		return ""
	}
	target := filepath.ToSlash(strings.TrimSpace(args[0]))
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "~/") || currentCwd == "" {
		return target
	}
	return filepath.ToSlash(filepath.Join(currentCwd, target))
}

func parseGitSubcommandAndDir(args []string, currentCwd string) (string, string) {
	effectiveDir := currentCwd
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-C" && i+1 < len(args) {
			effectiveDir = resolveSentinelCdTarget(effectiveDir, []string{args[i+1]})
			i++
			continue
		}
		if strings.HasPrefix(arg, "--work-tree=") {
			effectiveDir = resolveSentinelCdTarget(effectiveDir, []string{strings.TrimPrefix(arg, "--work-tree=")})
			continue
		}
		if (arg == "-c" || arg == "--git-dir" || arg == "--work-tree" || arg == "--namespace") && i+1 < len(args) {
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg, effectiveDir
	}
	return "", effectiveDir
}
