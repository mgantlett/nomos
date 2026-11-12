// Package astdebt implements the deterministic Definition of Done (DoD) quality gate pipeline.
package astdebt

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/treesitter"
)

// FileMetric records lines of code and cognitive debt score for a file (#FileMetric).
type FileMetric struct {
	Name string `json:"name"`
	LOC  int    `json:"loc"`
	CDS  int    `json:"cds"`
}

// FunctionMetric records complexity and LOC for an individual function (#FunctionMetric).
type FunctionMetric struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Name       string `json:"name"`
	Complexity int    `json:"complexity"`
	LOC        int    `json:"loc"`
}

// Thresholds captures the numerical quality bounds for gates (#Thresholds).
type Thresholds struct {
	MinCoverage               float64 `json:"min_coverage"`
	MinDocstringDensity       int     `json:"min_docstring_density"`
	MinTraceabilityDensity    int     `json:"min_traceability_density"`
	MaxCDS                    int     `json:"max_cds"`
	MaxComplexity             int     `json:"max_complexity"`
	MaxFuncLOC                int     `json:"max_func_loc"`
	MaxFileLOC                int     `json:"max_file_loc"`
	MaxFuncParams             int     `json:"max_func_params"`
	MinCloneLines             int     `json:"min_clone_lines"`
	MaxMagicStringDuplication int     `json:"max_magic_string_duplication"`
	MaxPackageFiles           int     `json:"max_package_files"`
}

// DefaultThresholds returns canonical statutory quality gate thresholds (@intent tree_sitter_ast_gate_spec.md #Thresholds).
func DefaultThresholds() Thresholds {
	return Thresholds{
		MinCoverage:               55.0,
		MinDocstringDensity:       10,
		MinTraceabilityDensity:    50,
		MaxCDS:                    45,
		MaxComplexity:             15,
		MaxFuncLOC:                60,
		MaxFileLOC:                400,
		MaxFuncParams:             8,
		MinCloneLines:             18,
		MaxMagicStringDuplication: 2,
		MaxPackageFiles:           40,
	}
}

// FunctionDebt captures cyclomatic complexity and cognitive debt score (#FunctionDebt).
type FunctionDebt struct {
	Name       string `json:"name"`
	File       string `json:"file"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	LOC        int    `json:"loc"`
	Complexity int    `json:"complexity"`
	DebtScore  int    `json:"debt_score"`
}

// FileDebtReport aggregates cognitive debt metrics across all functions in a file (#FileDebtReport).
type FileDebtReport struct {
	Path      string         `json:"path"`
	TotalLOC  int            `json:"total_loc"`
	DebtScore int            `json:"debt_score"`
	Functions []FunctionDebt `json:"functions"`
}

// WorkspaceDebtReport represents aggregate cognitive debt across analyzed source files (#WorkspaceDebtReport).
type WorkspaceDebtReport struct {
	TotalFiles int              `json:"total_files"`
	TotalDebt  int              `json:"total_debt"`
	CleanFiles int              `json:"clean_files"`
	DebtFiles  int              `json:"debt_files"`
	Files      []FileDebtReport `json:"files"`
}

// TokenEntropy represents lexical token probability and frequency distribution (#TokenEntropy).
type TokenEntropy struct {
	Token string  `json:"token"`
	Count int     `json:"count"`
	P     float64 `json:"p"`
}

// FileEntropyReport represents Shannon information entropy H(X) for a source file (#FileEntropyReport).
type FileEntropyReport struct {
	Path         string         `json:"path"`
	TotalTokens  int            `json:"total_tokens"`
	UniqueTokens int            `json:"unique_tokens"`
	Entropy      float64        `json:"entropy"`
	TopTokens    []TokenEntropy `json:"top_tokens"`
}

// WorkspaceEntropyReport aggregates Shannon information entropy across the workspace (#WorkspaceEntropyReport).
type WorkspaceEntropyReport struct {
	TotalFiles  int                 `json:"total_files"`
	TotalTokens int                 `json:"total_tokens"`
	MeanEntropy float64             `json:"mean_entropy"`
	Files       []FileEntropyReport `json:"files"`
}

// CloneOccurrence identifies the exact file location of a structural clone block (#CloneOccurrence).
type CloneOccurrence struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// CloneGroup represents a cluster of structurally duplicated code blocks (#CloneGroup).
type CloneGroup struct {
	Hash        string            `json:"hash"`
	LineCount   int               `json:"line_count"`
	Occurrences []CloneOccurrence `json:"occurrences"`
}

// CalculateFunctionDebt computes penalty points exceeding maximum complexity and line length.
func CalculateFunctionDebt(loc, complexity, maxLoc, maxComp int) int {
	debt := 0
	if complexity > maxComp {
		debt += (complexity - maxComp) * 10
	}
	if loc > maxLoc {
		debt += (loc - maxLoc) * 2
	}
	return debt
}

// CalculateShannonEntropy calculates information entropy H(X) = -sum(p * log2(p)).
func CalculateShannonEntropy(tokens []string) (float64, []TokenEntropy) {
	if len(tokens) == 0 {
		return 0.0, nil
	}

	freq := make(map[string]int)
	for _, tok := range tokens {
		freq[tok]++
	}

	total := float64(len(tokens))
	var entropy float64
	var tokenEntropies []TokenEntropy

	for tok, count := range freq {
		p := float64(count) / total
		entropy -= p * math.Log2(p)
		tokenEntropies = append(tokenEntropies, TokenEntropy{Token: tok, Count: count, P: p})
	}

	sort.Slice(tokenEntropies, func(i, j int) bool {
		return tokenEntropies[i].Count > tokenEntropies[j].Count
	})

	return entropy, tokenEntropies
}

// AnalyzeFileDebt extracts cognitive debt metrics from Go or polyglot source.
func AnalyzeFileDebt(filePath string, src []byte, maxComp, maxFuncLOC int) (*FileDebtReport, error) {
	if strings.HasSuffix(filePath, ".go") {
		return analyzeGoFileDebt(filePath, src, maxComp, maxFuncLOC)
	}
	lang := treesitter.DetectLanguage(filePath)
	if lang != "" {
		return analyzePolyglotFileDebt(lang, filePath, src, maxComp, maxFuncLOC)
	}
	return nil, types.NewNomosError(types.ErrCodeCore, "failed parsing source %s: unsupported format", filePath)
}

func analyzeGoFileDebt(filePath string, src []byte, maxComp, maxFuncLOC int) (*FileDebtReport, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "failed parsing source %s: %v", filePath, err)
	}

	var funcDebts []FunctionDebt
	totalDebt := 0

	for _, decl := range node.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		fd := inspectFuncDebt(fn, fset, filePath, maxComp, maxFuncLOC)
		funcDebts = append(funcDebts, fd)
		totalDebt += fd.DebtScore
	}

	totalLines := fset.Position(node.End()).Line
	return &FileDebtReport{
		Path:      filePath,
		TotalLOC:  totalLines,
		DebtScore: totalDebt,
		Functions: funcDebts,
	}, nil
}

func analyzePolyglotFileDebt(lang, filePath string, src []byte, maxComp, maxFuncLOC int) (*FileDebtReport, error) {
	engine, err := treesitter.GetDefaultEngine()
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "failed initializing treesitter engine: %v", err)
	}
	rep, err := engine.ParseAndEvaluate(context.Background(), filePath, src)
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeCore, "failed parsing source %s: %v", filePath, err)
	}
	var funcDebts []FunctionDebt
	totalDebt := 0
	for _, fn := range rep.Functions {
		debt := CalculateFunctionDebt(fn.LOC, fn.Complexity, maxFuncLOC, maxComp)
		funcDebts = append(funcDebts, FunctionDebt{
			Name:       fn.Name,
			File:       filePath,
			StartLine:  fn.Line,
			EndLine:    fn.Line + fn.LOC - 1,
			LOC:        fn.LOC,
			Complexity: fn.Complexity,
			DebtScore:  debt,
		})
		totalDebt += debt
	}
	totalLines := strings.Count(string(src), "\n") + 1
	return &FileDebtReport{
		Path:      filePath,
		TotalLOC:  totalLines,
		DebtScore: totalDebt,
		Functions: funcDebts,
	}, nil
}

func inspectFuncDebt(fn *ast.FuncDecl, fset *token.FileSet, filePath string, maxComp, maxFuncLOC int) FunctionDebt {
	start := fset.Position(fn.Pos()).Line
	end := fset.Position(fn.End()).Line
	loc := end - start + 1
	comp := CalculateCyclomaticComplexity(fn)
	debt := CalculateFunctionDebt(loc, comp, maxFuncLOC, maxComp)

	return FunctionDebt{
		Name:       fn.Name.Name,
		File:       filePath,
		StartLine:  start,
		EndLine:    end,
		LOC:        loc,
		Complexity: comp,
		DebtScore:  debt,
	}
}

// CalculateCyclomaticComplexity computes McCabe cyclomatic complexity for a Go function.
func CalculateCyclomaticComplexity(fn *ast.FuncDecl) int {
	if fn == nil || fn.Body == nil {
		return 1
	}

	comp := 1
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			comp++
		case *ast.BinaryExpr:
			if x.Op == token.LAND || x.Op == token.LOR {
				comp++
			}
		}
		return true
	})
	return comp
}
