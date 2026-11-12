package astdebt

import (
	"testing"
)

func TestCalculateFunctionDebt(t *testing.T) {
	// Within limits: 0 debt
	debt := CalculateFunctionDebt(40, 10, 60, 15)
	if debt != 0 {
		t.Errorf("expected 0 debt, got %d", debt)
	}

	// Exceeds complexity by 2 (comp 17 > 15): 2 * 10 = 20
	// Exceeds loc by 5 (loc 65 > 60): 5 * 2 = 10
	// Total = 30
	debt = CalculateFunctionDebt(65, 17, 60, 15)
	if debt != 30 {
		t.Errorf("expected debt 30, got %d", debt)
	}
}

func TestCalculateShannonEntropy(t *testing.T) {
	tokens := []string{"func", "main", "func", "return", "func"}
	entropy, topTokens := CalculateShannonEntropy(tokens)

	if entropy <= 0 {
		t.Errorf("expected positive entropy, got %f", entropy)
	}
	if len(topTokens) != 3 {
		t.Errorf("expected 3 unique tokens, got %d", len(topTokens))
	}
	if topTokens[0].Token != "func" || topTokens[0].Count != 3 {
		t.Errorf("expected 'func' with count 3, got %s with %d", topTokens[0].Token, topTokens[0].Count)
	}
}

func TestAnalyzeFileDebt(t *testing.T) {
	src := `package mathutils

func Simple() int {
	return 42
}

func Branchy(x int) int {
	if x > 0 {
		if x > 10 {
			return 1
		}
		return 2
	}
	return 0
}
`

	rep, err := AnalyzeFileDebt("mathutils.go", []byte(src), 15, 60)
	if err != nil {
		t.Fatalf("unexpected error analyzing debt: %v", err)
	}

	if rep.TotalLOC <= 0 {
		t.Errorf("expected total LOC > 0, got %d", rep.TotalLOC)
	}
	if len(rep.Functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(rep.Functions))
	}

	simpleFn := rep.Functions[0]
	if simpleFn.Complexity != 1 {
		t.Errorf("expected Simple complexity 1, got %d", simpleFn.Complexity)
	}

	branchyFn := rep.Functions[1]
	if branchyFn.Complexity != 3 {
		t.Errorf("expected Branchy complexity 3, got %d", branchyFn.Complexity)
	}
}
