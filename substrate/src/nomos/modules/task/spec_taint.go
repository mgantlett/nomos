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

var intentSpecRefRe = regexp.MustCompile(`@intent[^\n]*?\b([A-Za-z0-9_-]+\.md)\b`)

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
	if repoRoot == "" {
		return nil
	}
	specIndex := indexRepoMarkdownSpecs(repoRoot)
	if len(specIndex) == 0 {
		return nil
	}
	referenced := make(map[string]bool)
	for _, rel := range touchedFiles {
		collectFileIntentSpecRefs(repoRoot, rel, specIndex, referenced)
	}
	if strings.HasPrefix(strings.TrimSpace(twinKey), "INT-") {
		collectTwinCommitSpecs(repoRoot, strings.TrimSpace(twinKey), specIndex, referenced)
	}
	return hashReferencedSpecs(repoRoot, referenced)
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
		if !strings.HasPrefix(payload.TaskKey, "SUB-") {
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

func indexRepoMarkdownSpecs(repoRoot string) map[string]string {
	intentDir := filepath.Join(repoRoot, "intent")
	fi, err := os.Stat(intentDir)
	if err != nil || !fi.IsDir() {
		return nil
	}
	index := make(map[string]string)
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
		return index
	}
	return index
}

func collectFileIntentSpecRefs(repoRoot, relFile string, specIndex map[string]string, out map[string]bool) {
	full := filepath.Join(repoRoot, filepath.FromSlash(relFile))
	dir := filepath.Dir(full)
	entries, err := os.ReadDir(dir)
	if err != nil {
		scanSingleSourceSpecRefs(full, specIndex, out)
		return
	}
	for _, ent := range entries {
		if !ent.IsDir() && !strings.HasSuffix(ent.Name(), "_test.go") {
			scanSingleSourceSpecRefs(filepath.Join(dir, ent.Name()), specIndex, out)
		}
	}
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
