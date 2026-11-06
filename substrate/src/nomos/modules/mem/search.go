// Package mem implements the Nomos Mneme episodic and vector memory system.
package mem

import (
	"math"
	"sort"
	"strings"
	"time"
)

// SearchResult encapsulates a ranked memory match.
type SearchResult struct {
	Memory  *Memory `json:"memory"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

// SearchOptions encapsulates parameters for querying Nomos Mneme memory.
type SearchOptions struct {
	Limit            int
	Kind             MemoryKind
	Topic            string
	RepoID           string
	IncludeCrossRepo bool
	IncludeInactive  bool
	ReadOnly         bool
}

// Search queries memories and returns results ordered by descending relevance.
func (s *Store) Search(query string, limit int, kind MemoryKind, topic string) ([]*SearchResult, error) {
	return s.SearchWithOptions(query, SearchOptions{
		Limit:           limit,
		Kind:            kind,
		Topic:           topic,
		IncludeInactive: false,
	})
}

const repoLocalityBoostFactor = 1.25

// SearchWithOptions queries memories using explicit search configuration.
// @intent mneme_memory_spec.md #Memory
func (s *Store) SearchWithOptions(query string, opts SearchOptions) ([]*SearchResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 5
	}

	queryEmb, err := GenerateEmbedding(query)
	if err != nil {
		return nil, err
	}
	records, err := s.ListAll()
	if err != nil {
		return nil, err
	}

	results := scoreMatchingMemories(records, query, queryEmb, opts)
	sortSearchResults(results)
	results = applyGlobalAxiomSlotCap(results, opts, limit)

	if !opts.ReadOnly {
		for _, res := range results {
			if rErr := s.RecordAccess(res.Memory.ID); rErr != nil {
				continue
			}
		}
	}

	return results, nil
}

func scoreMatchingMemories(records []*Memory, query string, queryEmb []float32, opts SearchOptions) []*SearchResult {
	var results []*SearchResult
	for _, rec := range records {
		if !opts.IncludeInactive && rec.Status != "" && rec.Status != StatusActive {
			continue
		}
		if !matchesSearchFilter(rec, opts) {
			continue
		}
		baseScore := CombinedScore(query, queryEmb, rec.Content, rec.Embedding)
		if baseScore <= 0.05 {
			continue
		}
		decay := CalculateTemporalDecay(rec.CreatedAt, rec.Kind)
		reinforcement := 1.0 + 0.05*math.Min(float64(rec.AccessCount), 10.0)
		locality := repoLocalityMultiplier(rec, opts)
		results = append(results, &SearchResult{
			Memory:  rec,
			Score:   baseScore * decay * reinforcement * locality,
			Snippet: formatSnippet(rec.Content, 100),
		})
	}
	return results
}

func repoLocalityMultiplier(rec *Memory, opts SearchOptions) float64 {
	if opts.IncludeCrossRepo || strings.TrimSpace(opts.RepoID) == "" {
		return 1.0
	}
	if strings.EqualFold(strings.TrimSpace(rec.RepoID), strings.TrimSpace(opts.RepoID)) {
		return repoLocalityBoostFactor
	}
	return 1.0
}

func isUnscopedGlobalAxiom(rec *Memory) bool {
	if rec == nil || rec.Kind != KindAxiom {
		return false
	}
	repo := strings.ToLower(strings.TrimSpace(rec.RepoID))
	return repo == "" || repo == "global"
}

func applyGlobalAxiomSlotCap(sorted []*SearchResult, opts SearchOptions, limit int) []*SearchResult {
	if opts.IncludeCrossRepo || strings.TrimSpace(opts.RepoID) == "" || opts.Kind != "" {
		if len(sorted) > limit {
			return sorted[:limit]
		}
		return sorted
	}
	maxGlobal := limit / 3
	if maxGlobal < 1 {
		maxGlobal = 1
	}
	var selected, overflow []*SearchResult
	globalUsed := 0
	for _, item := range sorted {
		if isUnscopedGlobalAxiom(item.Memory) {
			if globalUsed < maxGlobal {
				selected = append(selected, item)
				globalUsed++
			} else {
				overflow = append(overflow, item)
			}
			continue
		}
		selected = append(selected, item)
	}
	for len(selected) < limit && len(overflow) > 0 {
		selected = append(selected, overflow[0])
		overflow = overflow[1:]
	}
	if len(selected) > limit {
		selected = selected[:limit]
	}
	sortSearchResults(selected)
	return selected
}

func matchesSearchFilter(m *Memory, opts SearchOptions) bool {
	if opts.Kind != "" && m.Kind != opts.Kind {
		return false
	}
	if opts.Topic != "" && !strings.EqualFold(m.Topic, opts.Topic) {
		return false
	}
	if opts.RepoID != "" && !opts.IncludeCrossRepo {
		clean := strings.ToLower(strings.TrimSpace(opts.RepoID))
		memRepo := strings.ToLower(strings.TrimSpace(m.RepoID))
		if memRepo != clean && memRepo != "global" && memRepo != "" {
			return false
		}
	}
	return true
}

// CalculateTemporalDecay computes an Ebbinghaus exponential decay factor based on age and kind.
func CalculateTemporalDecay(createdAt time.Time, kind MemoryKind) float64 {
	if createdAt.IsZero() {
		return 1.0
	}
	ageHours := time.Since(createdAt).Hours()
	if ageHours <= 0 {
		return 1.0
	}

	var lambda float64
	switch kind {
	case KindAxiom:
		lambda = 0.005
	case KindPattern:
		lambda = 0.006
	case KindPitfall:
		lambda = 0.008
	case KindReflection:
		lambda = 0.02
	case KindRelease, KindTask, KindCommit:
		lambda = 0.05
	default:
		lambda = 0.02
	}

	ageDays := ageHours / 24.0
	decay := math.Exp(-lambda * ageDays)
	if decay < 0.10 {
		return 0.10
	}
	return decay
}

func sortSearchResults(results []*SearchResult) {
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
}

func formatSnippet(content string, maxLen int) string {
	cleaned := strings.ReplaceAll(content, "\n", " ")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) <= maxLen {
		return cleaned
	}
	return cleaned[:maxLen] + "..."
}
