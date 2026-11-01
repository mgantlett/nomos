package schemas

import "strings"

// FunctionDebt captures cyclomatic complexity and cognitive debt score for an individual function.
#FunctionDebt: close({
	name:        string & strings.MinRunes(1)
	file:        string & strings.MinRunes(1)
	start_line:  int & >= 1
	end_line:    int & >= 1
	loc:         int & >= 0
	complexity:  int & >= 1
	debt_score:  int & >= 0
})

// FileDebtReport aggregates cognitive debt metrics across all functions in a single source file.
#FileDebtReport: close({
	path:        string & strings.MinRunes(1)
	total_loc:   int & >= 0
	debt_score:  int & >= 0
	functions:   [...#FunctionDebt]
})

// WorkspaceDebtReport represents aggregate cognitive debt across all analyzed source files.
#WorkspaceDebtReport: close({
	total_files: int & >= 0
	total_debt:  int & >= 0
	clean_files: int & >= 0
	debt_files:  int & >= 0
	files:       [...#FileDebtReport]
})

// TokenEntropy represents lexical token probability and frequency distribution.
#TokenEntropy: close({
	token: string & strings.MinRunes(1)
	count: int & >= 1
	p:     float & >= 0.0 & <= 1.0
})

// FileEntropyReport represents Shannon information entropy H(X) for a single source file.
#FileEntropyReport: close({
	path:          string & strings.MinRunes(1)
	total_tokens:  int & >= 0
	unique_tokens: int & >= 0
	entropy:       float & >= 0.0
	top_tokens:    [...#TokenEntropy]
})

// WorkspaceEntropyReport aggregates Shannon information entropy across the entire workspace.
#WorkspaceEntropyReport: close({
	total_files:  int & >= 0
	total_tokens: int & >= 0
	mean_entropy: float & >= 0.0
	files:        [...#FileEntropyReport]
})

// CloneOccurrence identifies the exact file location of a structural clone block.
#CloneOccurrence: close({
	file:       string & strings.MinRunes(1)
	start_line: int & >= 1
	end_line:   int & >= 1
})

// CloneGroup represents a cluster of structurally duplicated code blocks across files.
#CloneGroup: close({
	hash:        string & strings.MinRunes(1)
	line_count:  int & >= 1
	occurrences: [#CloneOccurrence, #CloneOccurrence, ...#CloneOccurrence]
})

// RefactoringSeam prescribes an AST extraction cut-line for a complex function.
#RefactoringSeam: close({
	start_line: int & >= 1
	end_line:   int & >= 1
	kind:       string & strings.MinRunes(1)
	complexity: int & >= 1
	loc:        int & >= 1
	inputs?:    [...string]
})

// FileDecompositionPlan ranks top function extraction candidates and baseline deltas for oversized files.
#FileDecompositionPlan: close({
	file:           string & strings.MinRunes(1)
	baseline_loc?:  int & >= 0
	baseline_cds?:  int & >= 0
	top_candidates: [...#FunctionDebt]
})
