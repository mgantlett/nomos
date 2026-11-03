// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"os"
	"path/filepath"
	"strings"
)

// goInstallInvocation captures the parsed flags and target packages of a 'go install' command segment.
type goInstallInvocation struct {
	pkgArgs       []string
	hasUnsafeFlag bool
}

type shellSegmentScanner struct {
	segments   [][]string
	curSeg     []string
	tok        strings.Builder
	subs       []string
	tokStarted bool
	inSingle   bool
	inDouble   bool
}

var goBuildBoolFlags = map[string]bool{
	"a": true, "asan": true, "buildvcs": true, "cover": true, "i": true,
	"linkshared": true, "modcacherw": true, "msan": true, "n": true,
	"race": true, "trimpath": true, "v": true, "work": true, "x": true,
}

var goBuildValueFlags = map[string]bool{
	"C": true, "asmflags": true, "buildmode": true, "compiler": true,
	"covermode": true, "coverpkg": true, "gccgoflags": true, "gcflags": true,
	"installsuffix": true, "ldflags": true, "mod": true, "modfile": true,
	"o": true, "overlay": true, "p": true, "pkgdir": true, "tags": true, "toolexec": true,
}

var unsafeGoInstallFlags = map[string]bool{
	"C": true, "modfile": true, "o": true, "overlay": true, "toolexec": true,
}

// parseShellCommandSegments splits a shell command line into unquoted command segments
// and extracts any embedded command substitutions inside double-quoted strings.
func parseShellCommandSegments(cmdLine string) ([][]string, []string) {
	var sc shellSegmentScanner
	for i := 0; i < len(cmdLine); i++ {
		i = sc.consumeByte(cmdLine, i)
	}
	sc.flushSegment()
	return sc.segments, sc.subs
}

func (sc *shellSegmentScanner) consumeByte(cmdLine string, i int) int {
	ch := cmdLine[i]
	if sc.inSingle {
		if ch == '\'' {
			sc.inSingle = false
		} else {
			sc.tok.WriteByte(ch)
		}
		return i
	}
	if sc.inDouble {
		nextIdx, sub := consumeDoubleQuoteByte(cmdLine, i, &sc.tok, &sc.inDouble)
		if sub != "" {
			sc.subs = append(sc.subs, sub)
		}
		return nextIdx
	}
	return sc.consumeUnquotedByte(cmdLine, i)
}

func (sc *shellSegmentScanner) consumeUnquotedByte(cmdLine string, i int) int {
	ch := cmdLine[i]
	if ch == '\\' && i+1 < len(cmdLine) {
		sc.tokStarted = true
		sc.tok.WriteByte(cmdLine[i+1])
		return i + 1
	}
	if ch == '\'' || ch == '"' {
		sc.tokStarted = true
		sc.inSingle = ch == '\''
		sc.inDouble = ch == '"'
		return i
	}
	if isShellSegmentDelimiter(ch) {
		sc.flushSegment()
		return i
	}
	if isShellWhitespace(ch) {
		sc.flushToken()
		return i
	}
	sc.tokStarted = true
	sc.tok.WriteByte(ch)
	return i
}

func (sc *shellSegmentScanner) flushToken() {
	if sc.tokStarted {
		sc.curSeg = append(sc.curSeg, sc.tok.String())
		sc.tok.Reset()
		sc.tokStarted = false
	}
}

func (sc *shellSegmentScanner) flushSegment() {
	sc.flushToken()
	if len(sc.curSeg) > 0 {
		sc.segments = append(sc.segments, sc.curSeg)
		sc.curSeg = nil
	}
}

func consumeDoubleQuoteByte(s string, i int, tok *strings.Builder, inDouble *bool) (int, string) {
	ch := s[i]
	if ch == '\\' && i+1 < len(s) {
		tok.WriteByte(s[i+1])
		return i + 1, ""
	}
	if ch == '"' {
		*inDouble = false
		return i, ""
	}
	if ch == '$' && i+1 < len(s) && s[i+1] == '(' {
		inner, endIdx := extractBalancedParenSub(s, i+2)
		return endIdx, inner
	}
	if ch == '`' {
		inner, endIdx := extractBacktickSub(s, i+1)
		return endIdx, inner
	}
	tok.WriteByte(ch)
	return i, ""
}

func extractBalancedParenSub(s string, start int) (string, int) {
	depth := 1
	for j := start; j < len(s); j++ {
		if s[j] == '\\' && j+1 < len(s) {
			j++
			continue
		}
		if s[j] == '(' {
			depth++
		} else if s[j] == ')' {
			depth--
			if depth == 0 {
				return s[start:j], j
			}
		}
	}
	return s[start:], len(s) - 1
}

func extractBacktickSub(s string, start int) (string, int) {
	for j := start; j < len(s); j++ {
		if s[j] == '\\' && j+1 < len(s) {
			j++
			continue
		}
		if s[j] == '`' {
			return s[start:j], j
		}
	}
	return s[start:], len(s) - 1
}

func isShellSegmentDelimiter(ch byte) bool {
	return ch == ';' || ch == '|' || ch == '&' || ch == '\n' || ch == '(' || ch == ')' || ch == '`'
}

func isShellWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\r'
}

// matchCdCommand resolves sequential directory changes ('cd <dir>') across chained shell segments.
func matchCdCommand(tokens []string, currentCwd string) (string, bool) {
	rest := stripLeadingEnvTokens(tokens)
	if len(rest) == 0 || rest[0] != "cd" {
		return "", false
	}
	args := rest[1:]
	for len(args) > 0 && (args[0] == "-L" || args[0] == "-P" || args[0] == "--") {
		args = args[1:]
	}
	if len(args) == 0 || args[0] == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return currentCwd, true
		}
		return home, true
	}
	return resolveCdArgPath(args[0], currentCwd), true
}

func resolveCdArgPath(target, currentCwd string) string {
	if strings.HasPrefix(target, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			target = filepath.Join(home, target[2:])
		}
	}
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	if currentCwd == "" {
		return filepath.Clean(target)
	}
	return filepath.Join(currentCwd, target)
}

// matchShellDashC extracts nested command scripts passed to 'sh -c', 'bash -lc', or 'zsh -c'.
func matchShellDashC(tokens []string) (string, bool) {
	rest := stripLeadingEnvTokens(tokens)
	if len(rest) < 3 {
		return "", false
	}
	base := filepath.Base(rest[0])
	if base != "sh" && base != "bash" && base != "zsh" {
		return "", false
	}
	for i := 1; i < len(rest)-1; i++ {
		arg := rest[i]
		if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
			continue
		}
		if strings.Contains(arg[1:], "c") {
			return rest[i+1], true
		}
	}
	return "", false
}

// matchGoInstallCommand checks whether a tokenized shell segment invokes 'go install'.
func matchGoInstallCommand(tokens []string) (goInstallInvocation, bool) {
	preUnsafe := hasUnsafeGoFlagsEnv(tokens)
	rest := stripLeadingEnvTokens(tokens)
	if len(rest) < 2 || (rest[0] != "go" && !strings.HasSuffix(rest[0], "/go")) {
		return goInstallInvocation{}, false
	}
	for i := 1; i < len(rest); i++ {
		tok := rest[i]
		if tok == "install" {
			inv := parseGoInstallArgs(rest[i+1:])
			if preUnsafe {
				inv.hasUnsafeFlag = true
			}
			return inv, true
		}
		if tok == "-C" {
			preUnsafe = true
			i++
			continue
		}
		if strings.HasPrefix(tok, "-C=") || strings.HasPrefix(tok, "--C=") {
			preUnsafe = true
			continue
		}
		if !strings.HasPrefix(tok, "-") {
			return goInstallInvocation{}, false
		}
		preUnsafe = true
	}
	return goInstallInvocation{}, false
}

func parseGoInstallArgs(args []string) goInstallInvocation {
	var inv goInstallInvocation
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			inv.pkgArgs = append(inv.pkgArgs, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			inv.pkgArgs = append(inv.pkgArgs, arg)
			continue
		}
		name, hasEq := parseFlagName(arg)
		if unsafeGoInstallFlags[name] {
			inv.hasUnsafeFlag = true
		}
		if goBuildBoolFlags[name] || hasEq {
			continue
		}
		if goBuildValueFlags[name] {
			if i+1 < len(args) {
				i++
			} else {
				inv.hasUnsafeFlag = true
			}
			continue
		}
		inv.hasUnsafeFlag = true
	}
	return inv
}

func parseFlagName(arg string) (string, bool) {
	trimmed := strings.TrimLeft(arg, "-")
	eq := strings.IndexByte(trimmed, '=')
	if eq < 0 {
		return trimmed, false
	}
	return trimmed[:eq], true
}

func stripLeadingEnvTokens(tokens []string) []string {
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if tok == "env" || filepath.Base(tok) == "env" {
			i++
			for i < len(tokens) && (tokens[i] == "-i" || tokens[i] == "-0") {
				i++
			}
			continue
		}
		if !isEnvAssignmentToken(tok) {
			break
		}
		i++
	}
	return tokens[i:]
}

func hasUnsafeGoFlagsEnv(tokens []string) bool {
	for _, tok := range tokens {
		if !isEnvAssignmentToken(tok) {
			if tok == "env" || filepath.Base(tok) == "env" || tok == "-i" || tok == "-0" {
				continue
			}
			break
		}
		if !strings.HasPrefix(tok, "GOFLAGS=") {
			continue
		}
		val := tok[len("GOFLAGS="):]
		if strings.Contains(val, "-overlay") || strings.Contains(val, "-modfile") || strings.Contains(val, "-C") {
			return true
		}
	}
	return false
}

func isEnvAssignmentToken(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	key := tok[:eq]
	for i := 0; i < len(key); i++ {
		ch := key[i]
		valid := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' || (i > 0 && ch >= '0' && ch >= '9')
		if !valid {
			return false
		}
	}
	return true
}
