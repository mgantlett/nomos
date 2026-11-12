// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func init() {
	RegisterGate("secret", func() vscope.Gate { return &SecretGate{} })
	RegisterGate("ip_leak", func() vscope.Gate { return &IPLeakGate{} })
	RegisterGate("broken_wire", func() vscope.Gate { return &BrokenWireGate{} })
	RegisterGate("config_immutability", func() vscope.Gate { return &ConfigImmutabilityGate{} })
}

// SecretGate detects unencrypted private keys, tokens, and credentials (Cell 7.1, Law 7).
type SecretGate struct{}

// Name returns the canonical display name of the secret gate.
func (g *SecretGate) Name() string { return "Secret & Credential Leak Invariant" }

// Evaluate scans workspace files for unencrypted secrets and private keys.
func (g *SecretGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	root := resolveSecurityRoot(ctx)
	leaks, err := scanSecretsScoped(ctx, root)
	if err != nil {
		return nil, err
	}
	if len(leaks) > 0 {
		return &vscope.GateResult{
			GateName: g.Name(), ProbeID: "secret", Law: 7, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d potential secret leak(s): %s", len(leaks), strings.Join(leaks, "; ")),
			Remedy:  "immediately revoke leaked keys and remove secrets from source files",
		}, nil
	}
	return &vscope.GateResult{
		GateName: g.Name(), ProbeID: "secret", Law: 7, Ring: RingSyntactic,
		Passed: true, Message: "zero unencrypted API keys or private certificates detected",
	}, nil
}

func resolveSecurityRoot(ctx *vscope.VerifyContext) string {
	if ctx != nil && ctx.WorktreePath != "" {
		return ctx.WorktreePath
	}
	if ctx != nil && ctx.RepoRoot != "" {
		return ctx.RepoRoot
	}
	return "."
}

func collectSecurityWalkFiles(ctx *vscope.VerifyContext, root string) ([]string, error) {
	return vscope.MemoizedWalkFiles(ctx, "secwalk:"+root, func() ([]string, error) {
		var files []string
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d == nil {
				return nil
			}
			if d.IsDir() {
				if shouldSkipSecurityDir(root, path, d.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			files = append(files, path)
			return nil
		})
		return files, err
	})
}

func scanSecrets(ctx *vscope.VerifyContext, root string) ([]string, error) {
	leaks := impact.ScanUnignoredEnvFiles(root)
	files, err := collectSecurityWalkFiles(ctx, root)
	if err != nil {
		return leaks, err
	}
	for _, path := range files {
		if isIgnoredSecretFile(path) || (impact.IsProhibitedEnvFileName(filepath.Base(path)) && impact.IsGitIgnoredPath(root, path)) {
			continue
		}
		data, rErr := vscope.MemoizedReadFile(ctx, path)
		if rErr != nil {
			continue
		}
		if label := impact.MatchSecretTokenLabel(string(data)); label != "" {
			rel, rErr := filepath.Rel(root, path)
			if rErr != nil {
				rel = path
			}
			leaks = append(leaks, fmt.Sprintf("%s (%s)", rel, label))
		}
	}
	return leaks, nil
}

func shouldSkipSecurityDir(root, path, name string) bool {
	if path == root {
		return false
	}
	return impact.IsSkippableSecurityDir(name) || isGitIgnoredBuildDir(root, path, name)
}

func isGitIgnoredBuildDir(root, path, name string) bool {
	ambiguous := map[string]bool{
		"build": true, "out": true, "coverage": true, "target": true,
	}
	return ambiguous[name] && impact.IsGitIgnoredPath(root, path)
}

func isIgnoredSecretFile(path string) bool {
	if strings.HasSuffix(path, "_test.go") {
		return true
	}
	ext := filepath.Ext(path)
	ignoredExts := map[string]bool{
		".sum": true, ".db": true, ".test": true, ".o": true,
		".a": true, ".so": true, ".dylib": true, ".dll": true,
		".pyc": true, ".wasm": true,
	}
	return ignoredExts[ext]
}

// IPLeakGate detects prohibited sovereign imports in open-core code (Cell 5.1, Law 5).
type IPLeakGate struct{}

// Name returns the canonical display name.
func (g *IPLeakGate) Name() string { return "Sovereign IP Leak & Dependency Gate (Law 7)" }

var bannedSovereignPrefixes = []string{
	"github.com/mgantlett/nomos-sovereign",
	"github.com/mgantlett/nomos-swarm",
	"github.com/mgantlett/gsi-sub-sov",
	"github.com/mgantlett/gsi-mem-sov",
	"github.com/mgantlett/gsi-inf-sov",
	"github.com/mgantlett/gsi-int-sov",
}

// Evaluate scans Go source files for banned sovereign imports.
func (g *IPLeakGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	root := resolveSecurityRoot(ctx)
	violations, err := scanIPLeaks(ctx, root)
	if err != nil {
		return nil, err
	}
	if len(violations) > 0 {
		return &vscope.GateResult{
			GateName: g.Name(), ProbeID: "ip_leak", Law: 5, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d prohibited sovereign import(s):\n    - %s", len(violations), strings.Join(violations, "\n    - ")),
			Remedy:  "remove sovereign package imports from open-core substrate",
		}, nil
	}
	return &vscope.GateResult{
		GateName: g.Name(), ProbeID: "ip_leak", Law: 5, Ring: RingSyntactic,
		Passed: true, Message: "zero sovereign IP leaks detected in open-core codebase",
	}, nil
}

func scanIPLeaks(ctx *vscope.VerifyContext, root string) ([]string, error) {
	var violations []string
	files, err := collectSecurityWalkFiles(ctx, root)
	if err != nil {
		return violations, err
	}
	for _, path := range files {
		if impact.IsVerificationSuiteFile(filepath.Base(path)) || !vscope.IsSupportedPolyglotSourceFile(path) {
			continue
		}
		rel, rErr := filepath.Rel(root, path)
		if rErr != nil {
			rel = path
		}
		for _, val := range impact.ExtractPolyglotFileImports(path) {
			if isBannedImport(val) {
				violations = append(violations, fmt.Sprintf("%s: imports %s", rel, val))
			}
		}
	}
	return violations, nil
}

func isBannedImport(val string) bool {
	for _, p := range bannedSovereignPrefixes {
		if val == p || strings.HasPrefix(val, p+"/") {
			return true
		}
	}
	return false
}

// BrokenWireGate verifies all relative markdown links resolve on disk (Cell 4.1, Law 4).
type BrokenWireGate struct{}

// Name returns the canonical display name.
func (g *BrokenWireGate) Name() string { return "Broken Wire & Link Invariant (Law 4)" }

// Evaluate scans markdown files for broken relative links.
func (g *BrokenWireGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	root := resolveSecurityRoot(ctx)
	broken, err := scanBrokenLinksScoped(ctx, root)
	if err != nil {
		return nil, err
	}
	if len(broken) > 0 {
		return &vscope.GateResult{
			GateName: g.Name(), ProbeID: "broken_wire", Law: 4, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d broken markdown link(s): %s", len(broken), strings.Join(broken, "; ")),
			Remedy:  "restore missing target files or update broken markdown links",
		}, nil
	}
	return &vscope.GateResult{
		GateName: g.Name(), ProbeID: "broken_wire", Law: 4, Ring: RingSyntactic,
		Passed: true, Message: "zero broken wires detected; all markdown links resolve to valid targets",
	}, nil
}

func scanBrokenLinks(ctx *vscope.VerifyContext, root string) ([]string, error) {
	var broken []string
	linkRegex := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	files, err := collectSecurityWalkFiles(ctx, root)
	if err != nil {
		return broken, err
	}
	for _, path := range files {
		if !strings.HasSuffix(path, ".md") {
			continue
		}
		data, rErr := vscope.MemoizedReadFile(ctx, path)
		if rErr == nil {
			broken = append(broken, checkLinksInContent(root, path, string(data), linkRegex)...)
		}
	}
	return broken, nil
}

func checkLinksInContent(root, path, content string, linkRegex *regexp.Regexp) []string {
	var broken []string
	dir := filepath.Dir(path)
	matches := linkRegex.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		target := strings.TrimSpace(m[1])
		if isIgnoredLinkTarget(target) {
			continue
		}
		clean := strings.Split(target, "#")[0]
		if clean == "" {
			continue
		}
		resolved := filepath.Join(dir, clean)
		if _, err := os.Stat(resolved); os.IsNotExist(err) {
			rel, rErr := filepath.Rel(root, path)
			if rErr != nil {
				rel = path
			}
			broken = append(broken, fmt.Sprintf("%s -> %s", rel, clean))
		}
	}
	return broken
}

func isIgnoredLinkTarget(target string) bool {
	return strings.HasPrefix(target, "http://") ||
		strings.HasPrefix(target, "https://") ||
		strings.HasPrefix(target, "file://") ||
		strings.HasPrefix(target, "mailto:") ||
		strings.HasPrefix(target, "#")
}

// ConfigImmutabilityGate ensures substrate tasks do not mutate intent config (Cell 7.1, Law 7).
type ConfigImmutabilityGate struct{}

// Name returns the canonical display name.
func (g *ConfigImmutabilityGate) Name() string {
	return "Intent Config Immutability Invariant (Law 4 & 7)"
}

// Evaluate verifies nomos.yaml is not mutated under active SUB-* tasks.
func (g *ConfigImmutabilityGate) Evaluate(ctx *vscope.VerifyContext) (*vscope.GateResult, error) {
	if ctx == nil || !strings.HasPrefix(ctx.TaskID, "SUB-") {
		return &vscope.GateResult{
			GateName: g.Name(), ProbeID: "config_immutability", Law: 7, Ring: RingSyntactic,
			Passed: true, Message: "task is not a substrate task; config mutation is permitted",
		}, nil
	}

	root := resolveSecurityRoot(ctx)
	cmd := workspace.SafeGitCommand(root, "status", "--porcelain")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	cfg := vscope.ResolveVerifyConfig(ctx)
	var intentRoots, substrateRoots []string
	if cfg != nil {
		intentRoots = cfg.Governance.QualityGates.IntentSourceRoots
		substrateRoots = cfg.Governance.QualityGates.SubstrateSourceRoots
	}

	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) < 3 {
			continue
		}
		p := filepath.ToSlash(strings.TrimSpace(trimmed[2:]))
		if isIntentConfigPath(p, intentRoots, substrateRoots) {
			return &vscope.GateResult{
				GateName: g.Name(), ProbeID: "config_immutability", Law: 7, Ring: RingSyntactic,
				Passed:  false,
				Message: fmt.Sprintf("substrate task %s illegally mutated %s", ctx.TaskID, p),
				Remedy:  "revert configuration changes or spawn an INT-* task to mutate configuration",
			}, nil
		}
	}

	return &vscope.GateResult{
		GateName: g.Name(), ProbeID: "config_immutability", Law: 7, Ring: RingSyntactic,
		Passed: true, Message: "nomos.yaml remains immutable under active substrate task",
	}, nil
}

func isConfigFile(base string) bool {
	return base == "nomos.yaml"
}

func pathInRoots(path string, roots []string, defaultRoot string) bool {
	if len(roots) == 0 {
		roots = []string{defaultRoot}
	}
	for _, r := range roots {
		cleanRoot := strings.Trim(filepath.ToSlash(r), "/")
		if cleanRoot != "" && (path == cleanRoot || strings.HasPrefix(path, cleanRoot+"/")) {
			return true
		}
	}
	return false
}

func isIntentConfigPath(path string, intentRoots, substrateRoots []string) bool {
	clean := strings.TrimPrefix(filepath.ToSlash(path), "/")
	if !isConfigFile(filepath.Base(clean)) {
		return false
	}
	// Root-level configuration is the canonical intent plane SSoT
	if !strings.Contains(clean, "/") {
		return true
	}
	if pathInRoots(clean, substrateRoots, "substrate") {
		return false
	}
	if pathInRoots(clean, intentRoots, "intent") {
		return true
	}
	return true
}
