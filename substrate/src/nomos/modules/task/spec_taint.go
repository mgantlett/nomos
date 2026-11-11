// Package task implements specification SHA-256 receipt stamping and
// downstream obligation taint detection across Git Notes release provenance (Law 4).
package task

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

var (
	intentSpecRefRe    = regexp.MustCompile(`@intent[^\n]*?\b([A-Za-z0-9_-]+\.md)\b`)
	taskVectorTokenRe  = regexp.MustCompile(`TC-[A-Z0-9]+-[0-9]+\b`)
	taskUnderVectorRe  = regexp.MustCompile(`TC_([A-Z0-9]+)_([0-9]+)\b`)
)

// StaleSpecObligation identifies an Intent specification modified after its latest SUB-* verification receipt (#StaleSpecObligation).
type StaleSpecObligation struct {
	SpecPath       string `json:"spec_path"`
	CurrentSHA256  string `json:"current_sha256"`
	VerifiedSHA256 string `json:"verified_sha256"`
	LastVerifiedBy string `json:"last_verified_by"`
}

// CollectReferencedSpecDigests resolves @intent specification files referenced by touchedFiles
// (and any specs modified by twinKey) and computes their 64-char SHA-256 digests (@intent nomos_intent_traceability_spec.md #StaleSpecObligation).
func CollectReferencedSpecDigests(repoRoot string, touchedFiles []string, twinKey string) map[string]string {
	return CollectReferencedSpecDigestsWithRoots(repoRoot, touchedFiles, twinKey, nil)
}

// CollectReferencedSpecDigestsWithRoots resolves @intent specification files and @verifies TC-* vectors across configured intentRoots (@intent nomos_intent_traceability_spec.md #StaleSpecObligation).
func CollectReferencedSpecDigestsWithRoots(repoRoot string, touchedFiles []string, twinKey string, intentRoots []string) map[string]string {
	if repoRoot == "" {
		return nil
	}
	specIndex := indexRepoMarkdownSpecs(repoRoot, intentRoots)
	if len(specIndex) == 0 {
		return nil
	}
	vectorToSpecs := indexSpecVectors(repoRoot, specIndex)
	referenced := make(map[string]bool)
	for _, rel := range touchedFiles {
		collectFileIntentSpecRefs(repoRoot, rel, specIndex, vectorToSpecs, referenced)
	}
	if strings.HasPrefix(strings.TrimSpace(twinKey), "INT-") {
		collectTwinCommitSpecs(repoRoot, strings.TrimSpace(twinKey), specIndex, referenced)
	}
	return hashReferencedSpecs(repoRoot, referenced)
}

// CollectContractSpecDigests hashes all *_spec.md files under specRoots when contract obligations are active (@intent nomos_intent_traceability_spec.md #StaleSpecObligation).
func CollectContractSpecDigests(repoRoot string, specRoots []string) map[string]string {
	if repoRoot == "" || (specRoots != nil && len(specRoots) == 0) {
		return nil
	}
	roots := specRoots
	if len(roots) == 0 {
		roots = []string{filepath.Join("intent", "architecture", "specs")}
	}
	referenced := make(map[string]bool)
	for _, r := range roots {
		collectRootSpecPaths(repoRoot, r, referenced)
	}
	return hashReferencedSpecs(repoRoot, referenced)
}

func collectRootSpecPaths(repoRoot, relRoot string, out map[string]bool) {
	target := filepath.Join(repoRoot, filepath.FromSlash(relRoot))
	fi, err := os.Stat(target)
	if err != nil {
		return
	}
	if !fi.IsDir() {
		if strings.HasSuffix(target, ".md") {
			if rel, rErr := filepath.Rel(repoRoot, target); rErr == nil {
				out[filepath.ToSlash(rel)] = true
			}
		}
		return
	}
	walkErr := filepath.Walk(target, func(path string, info os.FileInfo, wErr error) error {
		if wErr == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, "_spec.md") {
			if rel, rErr := filepath.Rel(repoRoot, path); rErr == nil {
				out[filepath.ToSlash(rel)] = true
			}
		}
		return nil
	})
	if walkErr != nil {
		return
	}
}

// ComputeStaleSpecObligations compares current specification SHA-256 digests on disk against
// the newest SUB-* verification receipts in refs/notes/nomos/releases (@intent nomos_intent_traceability_spec.md #StaleSpecObligation).
func ComputeStaleSpecObligations(repoRoot, ref string) ([]StaleSpecObligation, error) {
	if repoRoot == "" {
		return nil, nil
	}
	if ref == "" {
		ref = "refs/notes/nomos/releases"
	}
	latestReceipts := loadLatestSubSpecReceipts(repoRoot, ref)
	if len(latestReceipts) == 0 {
		return nil, nil
	}
	var specPaths []string
	for p := range latestReceipts {
		specPaths = append(specPaths, p)
	}
	sort.Strings(specPaths)

	var stale []StaleSpecObligation
	for _, relSpec := range specPaths {
		curHash, ok := computeSpecFileSHA256(filepath.Join(repoRoot, filepath.FromSlash(relSpec)))
		if !ok {
			continue
		}
		rec := latestReceipts[relSpec]
		if curHash != rec.digest {
			stale = append(stale, StaleSpecObligation{
				SpecPath:       relSpec,
				CurrentSHA256:  curHash,
				VerifiedSHA256: rec.digest,
				LastVerifiedBy: rec.taskKey,
			})
		}
	}
	return stale, nil
}

type recordedSpecReceipt struct {
	digest  string
	taskKey string
}

func loadLatestSubSpecReceipts(repoRoot, ref string) map[string]recordedSpecReceipt {
	cmd := workspace.SafeGitCommand(repoRoot, "log", "--notes="+ref, "--format=%N")
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	latest := make(map[string]recordedSpecReceipt)
	for _, rawLine := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var payload ReleaseNotePayload
		if uErr := json.Unmarshal([]byte(line), &payload); uErr != nil {
			continue
		}
		if len(payload.SpecDigests) == 0 {
			continue
		}
		recordUnseenSpecDigests(payload.TaskKey, payload.SpecDigests, latest)
	}
	return latest
}

func recordUnseenSpecDigests(taskKey string, digests map[string]string, latest map[string]recordedSpecReceipt) {
	for specPath, digest := range digests {
		norm := filepath.ToSlash(strings.TrimSpace(specPath))
		if norm == "" || !regexSHA256.MatchString(digest) {
			continue
		}
		if _, exists := latest[norm]; !exists {
			latest[norm] = recordedSpecReceipt{digest: digest, taskKey: taskKey}
		}
	}
}

func indexRepoMarkdownSpecs(repoRoot string, intentRoots []string) map[string]string {
	roots := intentRoots
	if len(roots) == 0 {
		roots = []string{"intent"}
	}
	index := make(map[string]string)
	for _, r := range roots {
		intentDir := filepath.Join(repoRoot, filepath.FromSlash(r))
		fi, err := os.Stat(intentDir)
		if err != nil || !fi.IsDir() {
			continue
		}
		walkErr := filepath.Walk(intentDir, func(path string, info os.FileInfo, wErr error) error {
			if wErr == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, ".md") {
				if rel, rErr := filepath.Rel(repoRoot, path); rErr == nil {
					slash := filepath.ToSlash(rel)
					index[filepath.Base(path)] = slash
					index[slash] = slash
				}
			}
			return nil
		})
		if walkErr != nil {
			continue
		}
	}
	if len(index) == 0 {
		return nil
	}
	return index
}

func indexSpecVectors(repoRoot string, specIndex map[string]string) map[string][]string {
	seenSpecs := make(map[string]bool)
	vecMap := make(map[string][]string)
	for _, relSpec := range specIndex {
		if seenSpecs[relSpec] {
			continue
		}
		seenSpecs[relSpec] = true
		raw, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relSpec)))
		if err != nil {
			continue
		}
		for _, tok := range taskVectorTokenRe.FindAllString(string(raw), -1) {
			vecMap[tok] = append(vecMap[tok], relSpec)
		}
	}
	return vecMap
}

func collectFileIntentSpecRefs(repoRoot, relFile string, specIndex map[string]string, vectorToSpecs map[string][]string, out map[string]bool) {
	cleanRel := filepath.ToSlash(strings.TrimSpace(relFile))
	if relSpec, ok := specIndex[cleanRel]; ok {
		out[relSpec] = true
		return
	}
	full := filepath.Join(repoRoot, filepath.FromSlash(relFile))
	if isTaskVerificationSuiteName(filepath.Base(full)) {
		scanTestVectorSpecRefs(full, vectorToSpecs, out)
		return
	}
	scanSingleSourceSpecRefs(full, specIndex, out)
}

func scanTestVectorSpecRefs(path string, vectorToSpecs map[string][]string, out map[string]bool) {
	raw, err := os.ReadFile(path)
	if err != nil || len(vectorToSpecs) == 0 {
		return
	}
	text := string(raw)
	for _, tok := range taskVectorTokenRe.FindAllString(text, -1) {
		for _, sp := range vectorToSpecs[tok] {
			out[sp] = true
		}
	}
	for _, m := range taskUnderVectorRe.FindAllStringSubmatch(text, -1) {
		if len(m) == 3 {
			norm := "TC-" + m[1] + "-" + m[2]
			for _, sp := range vectorToSpecs[norm] {
				out[sp] = true
			}
		}
	}
}

func isTaskVerificationSuiteName(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, "_test.go") ||
		strings.HasSuffix(lower, "_test.py") ||
		strings.HasSuffix(lower, "_spec.rb") ||
		strings.HasSuffix(lower, "_test.rb") ||
		strings.HasPrefix(lower, "test_") ||
		strings.Contains(lower, ".test.") ||
		strings.Contains(lower, ".spec.")
}

func scanSingleSourceSpecRefs(path string, specIndex map[string]string, out map[string]bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, m := range intentSpecRefRe.FindAllStringSubmatch(string(raw), -1) {
		if len(m) > 1 {
			if relSpec, ok := specIndex[m[1]]; ok {
				out[relSpec] = true
			}
		}
	}
}

func collectTwinCommitSpecs(repoRoot, twinKey string, specIndex map[string]string, out map[string]bool) {
	cmd := workspace.SafeGitCommand(repoRoot, "log", "--grep=\\["+twinKey+"\\]", "-n", "1", "--name-only", "--format=")
	raw, err := cmd.Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(raw), "\n") {
		clean := filepath.ToSlash(strings.TrimSpace(line))
		if relSpec, ok := specIndex[clean]; ok {
			out[relSpec] = true
		}
	}
}

func hashReferencedSpecs(repoRoot string, referenced map[string]bool) map[string]string {
	if len(referenced) == 0 {
		return nil
	}
	digests := make(map[string]string, len(referenced))
	for relSpec := range referenced {
		if h, ok := computeSpecFileSHA256(filepath.Join(repoRoot, filepath.FromSlash(relSpec))); ok {
			digests[relSpec] = h
		}
	}
	if len(digests) == 0 {
		return nil
	}
	return digests
}

func computeSpecFileSHA256(path string) (string, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), true
}
