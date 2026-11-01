package verify

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestValidateAntiShim(t *testing.T) {
	// Valid function
	valid := AntiShimDTO{Name: "DoWork", BodyStatementCount: 2, IsPanicShim: false}
	if err := ValidateAntiShim(valid); err != nil {
		t.Errorf("expected valid dto to pass: %v", err)
	}

	// Empty stub
	empty := AntiShimDTO{Name: "EmptyStub", BodyStatementCount: 0}
	if err := ValidateAntiShim(empty); err == nil {
		t.Errorf("expected empty stub to fail validation")
	}

	// Panic shim
	panicShim := AntiShimDTO{Name: "PanicStub", BodyStatementCount: 1, IsPanicShim: true}
	if err := ValidateAntiShim(panicShim); err == nil {
		t.Errorf("expected panic stub to fail validation")
	}

	// Forbidden literal shim
	forbid := AntiShimDTO{Name: "BadLiteral", BodyStatementCount: 1, ForbiddenLiteralShim: "/home/developer/"}
	if err := ValidateAntiShim(forbid); err == nil {
		t.Errorf("expected forbidden literal to fail validation")
	}

	// Trivial pass shim
	trivial := AntiShimDTO{Name: "Evaluate", BodyStatementCount: 1, IsTrivialPassShim: true}
	if err := ValidateAntiShim(trivial); err == nil {
		t.Errorf("expected trivial pass shim to fail validation")
	}
}

func TestScanFunctionAntiShim(t *testing.T) {
	src := `package test
func Real() {
	x := 1
	_ = x
}

func Stub() {}

func PanicTodo() {
	panic("TODO: implement me")
}

func Evaluate() (*GateResult, error) {
	return &GateResult{GateName: "Trivial", Passed: true}, nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", []byte(src), 0)
	if err != nil {
		t.Fatalf("failed parsing test file: %v", err)
	}

	var dtoArray []AntiShimDTO
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			dto := ScanFunctionAntiShim(fn)
			dtoArray = append(dtoArray, dto)
		}
	}

	if len(dtoArray) != 4 {
		t.Fatalf("expected 4 functions, got %d", len(dtoArray))
	}

	// Real
	if dtoArray[0].BodyStatementCount != 2 || dtoArray[0].IsPanicShim {
		t.Errorf("unexpected Real dto: %+v", dtoArray[0])
	}

	// Stub
	if dtoArray[1].BodyStatementCount != 0 {
		t.Errorf("expected Stub to have 0 statements, got %d", dtoArray[1].BodyStatementCount)
	}

	// PanicTodo
	if !dtoArray[2].IsPanicShim {
		t.Errorf("expected PanicTodo to be identified as panic shim")
	}

	// Trivial Evaluate shim
	if !dtoArray[3].IsTrivialPassShim {
		t.Errorf("expected Evaluate to be identified as trivial pass shim")
	}
}
