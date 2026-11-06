// Package treesitter provides deterministic polyglot AST verification and Cognitive Debt Scoring (CDS)
// using pure-Go Tree-sitter query extraction and the wazero WebAssembly runtime (Law 2 Non-Verbal Proof).
package treesitter

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed queries/*
var QueryFS embed.FS

// FunctionMetric captures per-function complexity and line count metrics.
type FunctionMetric struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Name       string `json:"name"`
	Complexity int    `json:"complexity"`
	LOC        int    `json:"loc"`
	Documented bool   `json:"documented"`
	EmptyBlock bool   `json:"empty_block,omitempty"`
}

// FileMetric captures per-file line count and cognitive debt metrics.
type FileMetric struct {
	Name string `json:"name"`
	LOC  int    `json:"loc"`
	CDS  int    `json:"cds"`
}

// GateEvaluationReport aggregates polyglot AST verification findings.
type GateEvaluationReport struct {
	Functions       []FunctionMetric `json:"functions"`
	Files           []FileMetric     `json:"files"`
	Documented      int              `json:"documented"`
	Total           int              `json:"total"`
	Density         int              `json:"density"`
	EmptyBlocks     []string         `json:"empty_blocks,omitempty"`
	SkippedTests    []string         `json:"skipped_tests,omitempty"`
	HeadingLevels   []int            `json:"heading_levels,omitempty"`
	OutlineVerified bool             `json:"outline_verified,omitempty"`
	Passed          bool             `json:"pass"`
}

// EngineThresholds holds DoD thresholds for polyglot evaluation.
type EngineThresholds struct {
	MaxComplexity       int
	MaxFuncLOC          int
	MaxFileLOC          int
	MaxCDS              int
	MinDocstringDensity int
}

// DefaultEngineThresholds returns canonical DoD thresholds.
func DefaultEngineThresholds() EngineThresholds {
	return EngineThresholds{
		MaxComplexity:       15,
		MaxFuncLOC:          60,
		MaxFileLOC:          400,
		MaxCDS:              45,
		MinDocstringDensity: 10,
	}
}

// CalculateCDS computes the mathematical Cognitive Debt Score for a file:
// CDS(S) = sum(max(0, M(F) - 10) * 2.0 + max(0, LOC(F) - 30) * 0.5) + max(0, 100 - Density) * 0.2
func CalculateCDS(functions []FunctionMetric, density int) int {
	var debt float64
	for _, fn := range functions {
		if fn.Complexity > 10 {
			debt += float64(fn.Complexity-10) * 2.0
		}
		if fn.LOC > 30 {
			debt += float64(fn.LOC-30) * 0.5
		}
	}
	if density < 100 {
		debt += float64(100-density) * 0.2
	}
	return int(debt)
}

// ValidateQueryFS verifies that all required language queries are embedded and syntactically valid.
func ValidateQueryFS() error {
	langs := []string{
		"go",
		"typescript",
		"python",
		"ruby",
		"svelte",
		"rust",
		"c",
		"cpp",
		"java",
		"kotlin",
		"csharp",
		"html",
		"css",
	}
	for _, lang := range langs {
		spec, err := LoadSCMQuery(lang)
		if err != nil || spec == nil || len(strings.TrimSpace(spec.RawSource)) == 0 {
			return fmt.Errorf("missing or invalid embedded query for %s: %w", lang, err)
		}
	}
	return nil
}
