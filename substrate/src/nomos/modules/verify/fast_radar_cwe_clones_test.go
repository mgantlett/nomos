package verify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectPolyglotCWESinks_AndFastRadarCloneHint(t *testing.T) {
	// 1. CWE-89 Python dynamic SQLi at execute sink
	vulnSQL := []byte("def search_users(conn, dept, sort_col):\n    q = f\"SELECT username FROM users WHERE dept = '{dept}' ORDER BY {sort_col}\"\n    return conn.execute(q).fetchall()\n")
	if hits := DetectPolyglotCWESinks("repo/users.py", vulnSQL); len(hits) == 0 || !strings.Contains(hits[0], "CWE-89") {
		t.Fatalf("expected CWE-89 breach, got: %v", hits)
	}

	// Safe parameterized + allowlisted SQL passes cleanly, and SQL transpiler without execute() passes cleanly
	safeSQL := []byte("ALLOWED = {'username': 'username'}\ndef search_users(conn, dept, sort_col):\n    col = ALLOWED.get(sort_col, 'username')\n    return conn.execute(f'SELECT username FROM users WHERE dept = ? ORDER BY {col}', (dept,)).fetchall()\n")
	if hits := DetectPolyglotCWESinks("repo/users.py", safeSQL); len(hits) != 0 {
		t.Fatalf("expected safe SQL to pass, got: %v", hits)
	}
	transpilerSQL := []byte("def render_sql(table, expr):\n    return f'SELECT {expr} FROM {table} WHERE id = {expr}'\n")
	if hits := DetectPolyglotCWESinks("sqlglot/generator.py", transpilerSQL); len(hits) != 0 {
		t.Fatalf("expected non-executing SQL string transpiler to pass, got: %v", hits)
	}

	// 2. CWE-22 unconfined path traversal vs confined relative_to
	vulnPath := []byte("from pathlib import Path\ndef read_tenant(base_dir, rel):\n    return (Path(base_dir) / rel).read_text()\n")
	if hits := DetectPolyglotCWESinks("storage/reports.py", vulnPath); len(hits) == 0 || !strings.Contains(hits[0], "CWE-22") {
		t.Fatalf("expected CWE-22 breach, got: %v", hits)
	}
	safePath := []byte("from pathlib import Path\ndef read_tenant(base_dir, rel):\n    b = Path(base_dir).resolve()\n    p = (b / rel).resolve()\n    p.relative_to(b)\n    return p.read_text()\n")
	if hits := DetectPolyglotCWESinks("storage/reports.py", safePath); len(hits) != 0 {
		t.Fatalf("expected confined path reader to pass, got: %v", hits)
	}

	// 3. Intra-package Type-2 clone + Go interface peer hint in EvaluateFastFileRadar
	dir := t.TempDir()
	fnBody := "package mux\n\ntype Router interface { Get(p string) }\n\nfunc MethodA(x int) int {\n\ts := 0\n\tfor i := 0; i < x; i++ {\n\t\tif i%2 == 0 {\n\t\t\ts += i\n\t\t} else {\n\t\t\ts -= 1\n\t\t}\n\t\tif s > 10 {\n\t\t\ts /= 2\n\t\t}\n\t\tif s < -5 {\n\t\t\ts = 0\n\t\t}\n\t\ts += 1\n\t}\n\treturn s\n}\n"
	dupBody := strings.ReplaceAll(fnBody, "MethodA", "MethodB")
	dupBody = strings.ReplaceAll(dupBody, "type Router interface { Get(p string) }\n\n", "")
	fA := filepath.Join(dir, "mux.go")
	fB := filepath.Join(dir, "tree.go")
	if err := os.WriteFile(fA, []byte(fnBody), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fB, []byte(dupBody), 0644); err != nil {
		t.Fatal(err)
	}
	res, err := EvaluateFastFileRadar(dir, fB)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(res.Breaches, "\n")
	if !strings.Contains(joined, "CloneGate") {
		t.Fatalf("expected intra-package Type-2 clone breach in FastFileRadar, got: %v", res.Breaches)
	}
	hint := ComputeFirstEditCoChangeHint(dir, "tree.go")
	if !strings.Contains(hint, "mux.go") {
		t.Fatalf("expected Go interface peer hint naming mux.go, got: %q", hint)
	}
}

func TestFormatRatchetedBreachWithRecipe_AndCWERecipes(t *testing.T) {
	locMsg := formatRatchetedBreachWithRecipe("mux.go", "file LOC", 928, 925, 925)
	if !strings.Contains(locMsg, "1-TURN RECIPE") || !strings.Contains(locMsg, "sibling file") {
		t.Fatalf("expected sibling-file recipe on above-ceiling file LOC breach, got: %q", locMsg)
	}
	fnMsg := formatRatchetedBreachWithRecipe("mux.go", "func routeHTTP LOC", 70, 65, 65)
	if !strings.Contains(fnMsg, "1-TURN RECIPE") {
		t.Fatalf("expected function extraction recipe on ratcheted func breach, got: %q", fnMsg)
	}
	cweHits := DetectPolyglotCWESinks("storage/reports.py", []byte("from pathlib import Path\ndef r(base_dir, rel):\n    return (Path(base_dir)/rel).read_text()\n"))
	if len(cweHits) == 0 || !strings.Contains(cweHits[0], "1-TURN RECIPE") {
		t.Fatalf("expected 1-turn CWE-22 recipe, got: %v", cweHits)
	}
}
