// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func init() {
	RegisterGate("secret", func() Gate { return &SecretGate{} })
	RegisterGate("ip_leak", func() Gate { return &IPLeakGate{} })
	RegisterGate("broken_wire", func() Gate { return &BrokenWireGate{} })
	RegisterGate("config_immutability", func() Gate { return &ConfigImmutabilityGate{} })
}

// SecretGate detects unencrypted private keys, tokens, and credentials (Cell 7.1, Law 7).
type SecretGate struct{}

// Name returns the canonical display name of the secret gate.
func (g *SecretGate) Name() string { return "Secret & Credential Leak Invariant" }

// Evaluate scans workspace files for unencrypted secrets and private keys.
func (g *SecretGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveSecurityRoot(ctx)
	leaks, err := scanSecrets(root)
	if err != nil {
		return nil, err
	}
	if len(leaks) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "secret", Law: 7, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d potential secret leak(s): %s", len(leaks), strings.Join(leaks, "; ")),
			Remedy:  "immediately revoke leaked keys and remove secrets from source files",
		}, nil
	}
	return &GateResult{
		GateName: g.Name(), ProbeID: "secret", Law: 7, Ring: RingSyntactic,
		Passed: true, Message: "zero unencrypted API keys or private certificates detected",
	}, nil
}

func resolveSecurityRoot(ctx *VerifyContext) string {
	if ctx != nil && ctx.WorktreePath != "" {
		return ctx.WorktreePath
	}
	if ctx != nil && ctx.RepoRoot != "" {
		return ctx.RepoRoot
	}
	return "."
}

func scanSecrets(root string) ([]string, error) {
	leaks := scanUnignoredEnvFiles(root)

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
		if isIgnoredSecretFile(path) || (isProhibitedEnvFileName(d.Name()) && isGitIgnoredPath(root, path)) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			if label := matchSecretTokenLabel(string(data)); label != "" {
				rel, rErr := filepath.Rel(root, path)
				if rErr != nil {
					rel = path
				}
				leaks = append(leaks, fmt.Sprintf("%s (%s)", rel, label))
			}
		}
		return nil
	})
	return leaks, err
}

func shouldSkipSecurityDir(root, path, name string) bool {
	if path == root {
		return false
	}
	return isSkippableSecurityDir(name) || isGitIgnoredBuildDir(root, path, name)
}

func isSkippableSecurityDir(name string) bool {
	skip := map[string]bool{
		".git": true, "node_modules": true, ".tmp": true, "worktrees": true,
		"vendor": true, ".venv": true, "dist": true, "__pycache__": true,
		".svelte-kit": true, ".next": true, ".nuxt": true, ".output": true,
		".turbo": true, ".parcel-cache": true, ".vitepress": true,
	}
	return skip[name]
}

func isGitIgnoredBuildDir(root, path, name string) bool {
	ambiguous := map[string]bool{
		"build": true, "out": true, "coverage": true, "target": true,
	}
	return ambiguous[name] && isGitIgnoredPath(root, path)
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
func (g *IPLeakGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveSecurityRoot(ctx)
	violations, err := scanIPLeaks(root)
	if err != nil {
		return nil, err
	}
	if len(violations) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "ip_leak", Law: 5, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d prohibited sovereign import(s):\n    - %s", len(violations), strings.Join(violations, "\n    - ")),
			Remedy:  "remove sovereign package imports from open-core substrate",
		}, nil
	}
	return &GateResult{
		GateName: g.Name(), ProbeID: "ip_leak", Law: 5, Ring: RingSyntactic,
		Passed: true, Message: "zero sovereign IP leaks detected in open-core codebase",
	}, nil
}

func scanIPLeaks(root string) ([]string, error) {
	fset := token.NewFileSet()
	var violations []string

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
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		node, pErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if pErr != nil {
			return nil
		}
		rel, rErr := filepath.Rel(root, path)
		if rErr != nil {
			rel = path
		}
		for _, imp := range node.Imports {
			if imp.Path != nil {
				val := strings.Trim(imp.Path.Value, "\"")
				if isBannedImport(val) {
					violations = append(violations, fmt.Sprintf("%s: imports %s", rel, val))
				}
			}
		}
		return nil
	})
	return violations, err
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
func (g *BrokenWireGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	root := resolveSecurityRoot(ctx)
	broken, err := scanBrokenLinks(root)
	if err != nil {
		return nil, err
	}
	if len(broken) > 0 {
		return &GateResult{
			GateName: g.Name(), ProbeID: "broken_wire", Law: 4, Ring: RingSyntactic,
			Passed:  false,
			Message: fmt.Sprintf("detected %d broken markdown link(s): %s", len(broken), strings.Join(broken, "; ")),
			Remedy:  "restore missing target files or update broken markdown links",
		}, nil
	}
	return &GateResult{
		GateName: g.Name(), ProbeID: "broken_wire", Law: 4, Ring: RingSyntactic,
		Passed: true, Message: "zero broken wires detected; all markdown links resolve to valid targets",
	}, nil
}

func scanBrokenLinks(root string) ([]string, error) {
	var broken []string
	linkRegex := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)

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
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			broken = append(broken, checkLinksInContent(root, path, string(data), linkRegex)...)
		}
		return nil
	})
	return broken, err
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
func (g *ConfigImmutabilityGate) Evaluate(ctx *VerifyContext) (*GateResult, error) {
	if ctx == nil || !strings.HasPrefix(ctx.TaskID, "SUB-") {
		return &GateResult{
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

	cfg := resolveVerifyConfig(ctx)
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
			return &GateResult{
				GateName: g.Name(), ProbeID: "config_immutability", Law: 7, Ring: RingSyntactic,
				Passed:  false,
				Message: fmt.Sprintf("substrate task %s illegally mutated %s", ctx.TaskID, p),
				Remedy:  "revert configuration changes or spawn an INT-* task to mutate configuration",
			}, nil
		}
	}

	return &GateResult{
		GateName: g.Name(), ProbeID: "config_immutability", Law: 7, Ring: RingSyntactic,
		Passed: true, Message: "nomos.yaml remains immutable under active substrate task",
	}, nil
}

func isConfigFile(base string) bool {
	return base == "nomos.yaml" || base == "nomos.yml" || base == ".nomos.yaml" || base == ".nomos.yml"
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
