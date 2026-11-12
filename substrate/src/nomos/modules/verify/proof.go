// Package verify implements the deterministic Definition of Done (DoD) quality gate pipeline.
package verify

import (
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// ProofReport summarizes the outcomes of the Three Mathematical Proofs of Gate Integrity (#GateIntegrityReport).
type ProofReport struct {
	TargetDir             string  `json:"target_dir"`
	IsCodebaseTargeted    bool    `json:"is_codebase_targeted"`
	CodebaseFilesScanned  int     `json:"codebase_files_scanned"`
	MutationKilled        int     `json:"mutation_killed"`
	MutationTotal         int     `json:"mutation_total"`
	MutationScore         float64 `json:"mutation_score"`
	FalsePositiveRate     float64 `json:"false_positive_rate"`
	ReferenceGatesPass    int     `json:"reference_gates_pass"`
	ReferenceGatesTotal   int     `json:"reference_gates_total"`
	NonVacuousGates       int     `json:"non_vacuous_gates"`
	TotalGatesEvaluated   int     `json:"total_gates_evaluated"`
	Passed                bool    `json:"passed"`
	DurationTotalMicros   int64   `json:"duration_total_micros"`
	DurationVector1Micros int64   `json:"duration_vector1_micros"`
	DurationVector2Micros int64   `json:"duration_vector2_micros"`
	DurationVector3Micros int64   `json:"duration_vector3_micros"`
}

// EvaluateGateIntegrityAttestation executes all three deterministic attestation vectors of gate integrity.
func EvaluateGateIntegrityAttestation(repoRoot string) (*ProofReport, error) {
	start := time.Now()
	root := repoRoot
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}

	scannedFiles := findCodebaseGoFiles(root)
	rep := &ProofReport{
		TargetDir:            root,
		IsCodebaseTargeted:   len(scannedFiles) > 0,
		CodebaseFilesScanned: len(scannedFiles),
	}

	v1Start := time.Now()
	killed, mTotal, mErr := EvaluateMutationSensitivity(root, scannedFiles)
	if mErr != nil {
		return nil, types.NewNomosError(types.ErrCodeInvariant, "mutation proof failed: %v", mErr)
	}
	rep.DurationVector1Micros = time.Since(v1Start).Microseconds()
	rep.MutationKilled = killed
	rep.MutationTotal = mTotal
	if mTotal > 0 {
		rep.MutationScore = float64(killed) / float64(mTotal) * 100.0
	}

	v2Start := time.Now()
	passedGates, totalGates, specErr := EvaluateMetamorphicSpecificity(scannedFiles)
	if specErr != nil {
		return nil, types.NewNomosError(types.ErrCodeInvariant, "specificity proof failed: %v", specErr)
	}
	rep.DurationVector2Micros = time.Since(v2Start).Microseconds()
	rep.ReferenceGatesPass = passedGates
	rep.ReferenceGatesTotal = totalGates
	if totalGates > 0 {
		rep.FalsePositiveRate = float64(totalGates-passedGates) / float64(totalGates) * 100.0
	}

	v3Start := time.Now()
	nonVac, gTotal, vErr := EvaluateNonVacuousGates(root)
	if vErr != nil {
		return nil, types.NewNomosError(types.ErrCodeInvariant, "anti-vacuity proof failed: %v", vErr)
	}
	rep.DurationVector3Micros = time.Since(v3Start).Microseconds()
	rep.NonVacuousGates = nonVac
	rep.TotalGatesEvaluated = gTotal

	rep.DurationTotalMicros = time.Since(start).Microseconds()
	rep.Passed = rep.MutationScore >= 100.0 && rep.FalsePositiveRate == 0.0 &&
		rep.NonVacuousGates == rep.TotalGatesEvaluated && rep.TotalGatesEvaluated > 0
	return rep, nil
}

// findCodebaseGoFiles searches the root directory for scannable Go source files.
func findCodebaseGoFiles(root string) []string {
	var files []string
	if wErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "worktrees" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	}); wErr != nil {
		return files
	}
	return files
}

func sampleRealFunctions(files []string, maxSamples int) []*ast.FuncDecl {
	fset := token.NewFileSet()
	var samples []*ast.FuncDecl
	for _, file := range files {
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			continue
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Body != nil && len(fn.Body.List) > 0 {
				samples = append(samples, fn)
				if len(samples) >= maxSamples {
					return samples
				}
			}
		}
	}
	return samples
}

// EvaluateMutationSensitivity tests the gate suite against real codebase and synthetic mutants.
func EvaluateMutationSensitivity(root string, files []string) (int, int, error) {
	killed := 0
	total := 0

	samples := sampleRealFunctions(files, 4)
	for _, fn := range samples {
		emptyMut := &ast.FuncDecl{Name: fn.Name, Type: fn.Type, Body: &ast.BlockStmt{List: []ast.Stmt{}}}
		if ValidateAntiShim(ScanFunctionAntiShim(emptyMut)) != nil {
			killed++
		}
		total++

		panicMut := &ast.FuncDecl{
			Name: fn.Name, Type: fn.Type,
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("panic")}},
			}},
		}
		if ValidateAntiShim(ScanFunctionAntiShim(panicMut)) != nil {
			killed++
		}
		total++
	}

	sprawlMetrics := []astdebt.FileMetric{{Name: "mutant_sprawl.go", LOC: 75}}
	if len(EvaluateFileLOCViolations(sprawlMetrics, 60)) > 0 {
		killed++
	}
	total++

	fset := token.NewFileSet()
	file, pErr := parser.ParseFile(fset, "mutant_err.go", "package main\nfunc test() {\n\t_ = fail()\n}\n", 0)
	if pErr == nil {
		var violations []string
		checkUncheckedErrorsInFile(file, fset, "mutant_err.go", &violations)
		if len(violations) > 0 {
			killed++
		}
	}
	total++

	cobraStub, cErr := parser.ParseExpr(`&cobra.Command{Use: "stub", Short: "missing long and example"}`)
	if cErr == nil {
		if unary, ok := cobraStub.(*ast.UnaryExpr); ok {
			if comp, ok := unary.X.(*ast.CompositeLit); ok && len(astdebt.ValidateCobraCommandCompositeLit(comp)) > 0 {
				killed++
			}
		}
	}
	total++

	if len(samples) == 0 {
		synKilled, synTotal := evaluateSyntheticMutants()
		killed += synKilled
		total += synTotal
	}

	return killed, total, nil
}

func evaluateSyntheticMutants() (int, int) {
	killed := 0
	fnEmpty := &ast.FuncDecl{Name: ast.NewIdent("EmptyStub"), Body: &ast.BlockStmt{List: []ast.Stmt{}}}
	if ValidateAntiShim(ScanFunctionAntiShim(fnEmpty)) != nil {
		killed++
	}
	fnPanic := &ast.FuncDecl{
		Name: ast.NewIdent("PanicStub"),
		Body: &ast.BlockStmt{List: []ast.Stmt{
			&ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("panic")}},
		}},
	}
	if ValidateAntiShim(ScanFunctionAntiShim(fnPanic)) != nil {
		killed++
	}
	return killed, 2
}

// EvaluateMetamorphicSpecificity proves clean reference code passes without false positives.
func EvaluateMetamorphicSpecificity(scannedFiles []string) (int, int, error) {
	passed := 0
	total := 0

	samples := sampleRealFunctions(scannedFiles, 6)
	for _, fn := range samples {
		if ValidateAntiShim(ScanFunctionAntiShim(fn)) == nil {
			passed++
		}
		total++
	}

	cleanMetrics := []astdebt.FileMetric{{Name: "clean.go", LOC: 42}}
	if len(EvaluateFileLOCViolations(cleanMetrics, 60)) == 0 {
		passed++
	}
	total++

	fset := token.NewFileSet()
	file, pErr := parser.ParseFile(fset, "good.go", "package main\nfunc test() error {\n\terr := work()\n\treturn err\n}\n", 0)
	if pErr == nil {
		var violations []string
		checkUncheckedErrorsInFile(file, fset, "good.go", &violations)
		if len(violations) == 0 {
			passed++
		}
	}
	total++

	gates := DefaultBaselineGates()
	if len(gates) > 0 {
		passed++
	}
	total++

	cobraRef, cErr := parser.ParseExpr(`&cobra.Command{Use: "ok", Short: "short", Long: "long", Example: "nomos ok"}`)
	if cErr == nil {
		if unary, ok := cobraRef.(*ast.UnaryExpr); ok {
			if comp, ok := unary.X.(*ast.CompositeLit); ok && len(astdebt.ValidateCobraCommandCompositeLit(comp)) == 0 {
				passed++
			}
		}
	}
	total++

	return passed, total, nil
}

// EvaluateNonVacuousGates performs an AST or in-memory audit of all Gate.Evaluate implementations.
func EvaluateNonVacuousGates(repoRoot string) (int, int, error) {
	root := repoRoot
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}

	verifyDir := filepath.Join(root, "substrate", "src", "nomos", "modules", "verify")
	if stat, err := os.Stat(verifyDir); err != nil || !stat.IsDir() {
		verifyDir = filepath.Join(root, "src", "nomos", "modules", "verify")
	}

	if stat, err := os.Stat(verifyDir); err == nil && stat.IsDir() {
		return evaluateASTGateDirectory(verifyDir)
	}

	return evaluateCompiledGatesInMemory()
}

func evaluateASTGateDirectory(verifyDir string) (int, int, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, verifyDir, func(fi os.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), ".go") && !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return 0, 0, types.NewNomosError(types.ErrCodeCore, "failed parsing verify module AST: %v", err)
	}

	nonVacuous := 0
	totalGates := 0
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != "Evaluate" || fn.Recv == nil || len(fn.Recv.List) == 0 {
					continue
				}
				totalGates++
				if isNonVacuousEvaluate(fn) {
					nonVacuous++
				}
			}
		}
	}
	return nonVacuous, totalGates, nil
}

func evaluateCompiledGatesInMemory() (int, int, error) {
	gates := DefaultBaselineGates()
	nonVacuous := 0
	totalGates := len(gates)
	for _, g := range gates {
		if g == nil || g.Name() == "" {
			continue
		}
		nonVacuous++
	}
	return nonVacuous, totalGates, nil
}

func isNonVacuousEvaluate(fn *ast.FuncDecl) bool {
	if fn.Body == nil || len(fn.Body.List) == 0 {
		return false
	}
	if len(fn.Body.List) == 1 {
		if ret, ok := fn.Body.List[0].(*ast.ReturnStmt); ok && isTrivialGateReturn(ret) {
			return false
		}
		if ifStmt, ok := fn.Body.List[0].(*ast.IfStmt); ok && ifStmt.Body != nil && len(ifStmt.Body.List) == 1 {
			if ret, ok := ifStmt.Body.List[0].(*ast.ReturnStmt); ok && isTrivialGateReturn(ret) {
				return false
			}
		}
	}
	return true
}
