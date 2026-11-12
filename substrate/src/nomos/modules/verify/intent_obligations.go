// Package verify implements executable TC-* obligation binding, bijective specification
// struct signature AST parity, orphan test detection, and normative surjection (Law 2, Law 4, Law 7).
package verify

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
)

var (
	obligationVectorRe = regexp.MustCompile(`TC-[A-Z0-9]+-[0-9]+\b`)
	underscoreVectorRe = regexp.MustCompile(`TC_([A-Z0-9]+)_([0-9]+)\b`)
)

// VerifyExecutableObligations checks that TC-* vectors declared in Markdown tables
// across specPaths appear in repository verification suites (@intent nomos_intent_traceability_spec.md #ContractsConfig).
func VerifyExecutableObligations(root string, specPaths []string) []string {
	return verifyExecutableObligationsWithDelta(root, specPaths, collectActiveSiblingTwinDeltas(root, extractWorktreeSubTaskKey(root)))
}

func verifyExecutableObligationsWithDelta(root string, specPaths []string, delta *impact.SiblingDelta) []string {
	resolvedSpecs := resolveObligationSpecFiles(root, specPaths)
	if len(resolvedSpecs) == 0 {
		return nil
	}
	boundVectors := collectVerifiedVectorRefs(root)
	var issues []string
	for _, specFile := range resolvedSpecs {
		relSpec := toRelSlash(root, specFile)
		for _, vec := range extractSpecTableVectors(specFile) {
			if !boundVectors[vec] && !delta.HasAddedTC(vec) {
				issues = append(issues, fmt.Sprintf("%s -> %s", relSpec, vec))
			}
		}
	}
	return issues
}

// VerifySpecTypeSignatures checks Go and polyglot contract types in fenced specification code blocks
// against compiled Go AST and polyglot source declarations (@intent nomos_intent_traceability_spec.md #ContractsConfig).
func VerifySpecTypeSignatures(root, targetDir string, specPaths []string) []string {
	return verifySpecTypeSignaturesWithDelta(root, targetDir, specPaths, collectActiveSiblingTwinDeltas(root, extractWorktreeSubTaskKey(root)))
}

func verifySpecTypeSignaturesWithDelta(root, targetDir string, specPaths []string, delta *impact.SiblingDelta) []string {
	resolvedSpecs := resolveObligationSpecFiles(root, specPaths)
	if len(resolvedSpecs) == 0 {
		return nil
	}
	var issues []string
	if targetDir != "" {
		scanDir := resolveGoStructScanDir(root, targetDir)
		compiled := impact.CollectCompiledStructIndex(scanDir)
		if len(compiled.ByPkgType) > 0 {
			for _, specFile := range resolvedSpecs {
				if raw, err := os.ReadFile(specFile); err == nil {
					relSpec := toRelSlash(root, specFile)
					specIdx := impact.ExtractSpecStructIndex(string(raw))
					issues = append(issues, impact.DiffSpecStructsBijective(relSpec, specIdx, compiled, delta)...)
				}
			}
		}
	}
	issues = append(issues, verifyPolyglotSpecSignatures(root, resolvedSpecs)...)
	return issues
}

// VerifyNoOrphanedTestObligations checks Go verification suites for @verifies TC-* annotations absent from specification tables
// (@intent nomos_intent_traceability_spec.md #ContractsConfig).
func VerifyNoOrphanedTestObligations(root string, specPaths []string) []string {
	declared := collectAllDeclaredRepoSpecVectors(root, specPaths)
	if len(declared) == 0 {
		return nil
	}
	return impact.ScanOrphanedGoTestObligations(root, declared)
}

// VerifyNormativeRegistrySurjection verifies 100% of self-hosted gate ProbeIDs, config structs, CLI flags, and ERR_* codes appear in specs.
// @intent intent.md #BoundaryConfig
// @intent go_ssot_and_json_schema_spec.md #EcosystemConfig
// @intent nomos_hermetic_worktree_spec.md #NormativeSurfaceDelta
// @intent task_lifecycle_cli_spec.md #ReleaseNotePayload
// @intent nomos_intent_traceability_spec.md #ContractsConfig
// @intent mneme_memory_spec.md #Memory
// @intent nomos_controlled_technical_english_spec.md #NCTEPolicy
// @intent software_physics_engine_spec.md #CapabilityMetrics
func VerifyNormativeRegistrySurjection(root string, specPaths []string) []string {
	resolved := resolveAllCanonicalRepoSpecs(root, specPaths)
	if len(resolved) == 0 {
		return nil
	}
	var sb strings.Builder
	for _, f := range resolved {
		if raw, err := os.ReadFile(f); err == nil {
			sb.Write(raw)
			sb.WriteByte('\n')
		}
	}
	return impact.VerifyRepoNormativeSurjection(root, sb.String())
}

func resolveGoStructScanDir(root, targetDir string) string {
	subDir := filepath.Join(root, "substrate")
	if fi, err := os.Stat(subDir); err == nil && fi.IsDir() {
		return subDir
	}
	return targetDir
}

func collectAllDeclaredRepoSpecVectors(root string, specPaths []string) map[string]bool {
	declared := make(map[string]bool)
	for _, f := range resolveAllCanonicalRepoSpecs(root, specPaths) {
		for _, vec := range extractSpecTableVectors(f) {
			declared[vec] = true
		}
	}
	return declared
}

func resolveAllCanonicalRepoSpecs(root string, specPaths []string) []string {
	combined := make([]string, 0, len(specPaths)+2)
	combined = append(combined, specPaths...)
	combined = append(combined, filepath.Join("intent", "intent.md"), filepath.Join("intent", "architecture", "specs"))
	return resolveObligationSpecFiles(root, combined)
}

func resolveObligationSpecFiles(root string, specPaths []string) []string {
	if specPaths != nil && len(specPaths) == 0 {
		return nil
	}
	roots := specPaths
	if len(roots) == 0 {
		roots = []string{filepath.Join("intent", "architecture", "specs")}
	}
	seen := make(map[string]bool)
	var files []string
	for _, entry := range roots {
		full := entry
		if !filepath.IsAbs(full) {
			full = filepath.Join(root, entry)
		}
		files = collectMarkdownSpecs(full, seen, files)
	}
	sort.Strings(files)
	return files
}

func collectMarkdownSpecs(target string, seen map[string]bool, acc []string) []string {
	fi, err := os.Stat(target)
	if err != nil {
		return acc
	}
	if !fi.IsDir() {
		if strings.HasSuffix(target, ".md") && !seen[target] {
			seen[target] = true
			acc = append(acc, target)
		}
		return acc
	}
	walkErr := filepath.Walk(target, func(path string, info os.FileInfo, wErr error) error {
		if wErr == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, "_spec.md") && !seen[path] {
			seen[path] = true
			acc = append(acc, path)
		}
		return nil
	})
	if walkErr != nil {
		return acc
	}
	return acc
}

func extractSpecTableVectors(specFile string) []string {
	raw, err := os.ReadFile(specFile)
	if err != nil {
		return nil
	}
	return impact.ExtractMarkdownTableVectors(string(raw))
}

func collectVerifiedVectorRefs(root string) map[string]bool {
	refs := make(map[string]bool)
	walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if impact.IsSkippedObligationDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if impact.IsVerificationSuiteFile(info.Name()) {
			scanFileVectorRefs(path, refs)
		}
		return nil
	})
	if walkErr != nil {
		return refs
	}
	return refs
}

func scanFileVectorRefs(path string, refs map[string]bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	text := string(raw)
	for _, tok := range obligationVectorRe.FindAllString(text, -1) {
		refs[tok] = true
	}
	for _, m := range underscoreVectorRe.FindAllStringSubmatch(text, -1) {
		if len(m) == 3 {
			refs[fmt.Sprintf("TC-%s-%s", m[1], m[2])] = true
		}
	}
}

func toRelSlash(root, fullPath string) string {
	if rel, err := filepath.Rel(root, fullPath); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(fullPath)
}
