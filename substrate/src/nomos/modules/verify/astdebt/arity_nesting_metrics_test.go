package astdebt

import "testing"

func TestSummarizeFileArityAndNesting_GoAndPolyglot(t *testing.T) {
	goSrc := []byte(`package sample

func ManyParams(a, b, c, d, e, f, g, h, i int) int {
	if a > 0 {
		for j := 0; j < b; j++ {
			if c > 0 {
				switch d {
				case 1:
					if e > 0 {
						for k := 0; k < f; k++ {
							return k
						}
					}
				}
			}
		}
	} else if g > 0 {
		return g
	}
	return i
}
`)
	maxP, maxN, highA, deepN := SummarizeFileArityAndNesting("sample.go", goSrc, 8, 5)
	if maxP != 9 || highA != 1 {
		t.Fatalf("expected maxParams=9 highArity=1, got maxParams=%d highArity=%d", maxP, highA)
	}
	if maxN != 6 || deepN != 1 {
		t.Fatalf("expected maxNesting=6 deepNesting=1, got maxNesting=%d deepNesting=%d", maxN, deepN)
	}

	pySrc := []byte(`def orchestrate(a, b, c, d, e, f, g, h, i):
    if a:
        for x in b:
            if c:
                while d:
                    if e:
                        for y in f:
                            return y
    elif g:
        return g
    return i
`)
	pyP, pyN, pyHigh, pyDeep := SummarizeFileArityAndNesting("worker.py", pySrc, 8, 5)
	if pyP != 9 || pyHigh != 1 {
		t.Fatalf("expected python maxParams=9 highArity=1, got %d / %d", pyP, pyHigh)
	}
	if pyN != 6 || pyDeep != 1 {
		t.Fatalf("expected python maxNesting=6 deepNesting=1, got %d / %d", pyN, pyDeep)
	}

	tsSrc := []byte(`export function dispatch(a: number, b: number, c: number, d: number, e: number, f: number, g: number, h: number, i: number) {
  if (a > 0) {
    for (let j = 0; j < b; j++) {
      if (c > 0) {
        while (d > 0) {
          if (e > 0) {
            if (f > 0) {
              return g;
            }
          }
        }
      }
    }
  }
  return i;
}
`)
	tsP, tsN, tsHigh, tsDeep := SummarizeFileArityAndNesting("app.ts", tsSrc, 8, 5)
	if tsP != 9 || tsHigh != 1 || tsN != 6 || tsDeep != 1 {
		t.Fatalf("expected ts 9/1/6/1, got %d/%d/%d/%d", tsP, tsHigh, tsN, tsDeep)
	}
}
