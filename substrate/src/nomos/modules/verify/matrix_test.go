package verify

import "testing"

func TestCanonicalMatrix(t *testing.T) {
	if CanonicalMatrix.Laws != 7 {
		t.Errorf("expected 7 laws, got %d", CanonicalMatrix.Laws)
	}
	if CanonicalMatrix.Rings != 3 {
		t.Errorf("expected 3 rings, got %d", CanonicalMatrix.Rings)
	}
	if CanonicalMatrix.Cells != 21 {
		t.Errorf("expected 21 cells, got %d", CanonicalMatrix.Cells)
	}
}

func TestCanonicalLaws(t *testing.T) {
	if len(CanonicalLaws) != 7 {
		t.Fatalf("expected 7 canonical laws, got %d", len(CanonicalLaws))
	}
	for i, law := range CanonicalLaws {
		if law.Law != i+1 {
			t.Errorf("law index mismatch: expected %d, got %d", i+1, law.Law)
		}
		if law.Name == "" || law.Domain == "" || law.Description == "" {
			t.Errorf("incomplete law metadata for law %d", law.Law)
		}
	}
}

func TestDefaultBaselineCells(t *testing.T) {
	cells := DefaultBaselineCells()
	if len(cells) != 21 {
		t.Fatalf("expected exactly 21 cells, got %d", len(cells))
	}

	lawCounts := make(map[int]int)
	ringCounts := make(map[string]int)

	for _, c := range cells {
		lawCounts[c.Law]++
		ringCounts[c.Ring]++
		if c.ID == "" || c.Name == "" || c.Remedy == "" {
			t.Errorf("incomplete cell specification in cell %s", c.ID)
		}
	}

	for law := 1; law <= 7; law++ {
		if lawCounts[law] != 3 {
			t.Errorf("law %d has %d cells; expected 3", law, lawCounts[law])
		}
	}

	if ringCounts[RingSyntactic] != 7 {
		t.Errorf("expected 7 syntactic cells, got %d", ringCounts[RingSyntactic])
	}
	if ringCounts[RingDynamic] != 7 {
		t.Errorf("expected 7 dynamic cells, got %d", ringCounts[RingDynamic])
	}
	if ringCounts[RingCryptographic] != 7 {
		t.Errorf("expected 7 cryptographic cells, got %d", ringCounts[RingCryptographic])
	}
}
