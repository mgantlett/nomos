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

// @verifies TC-TS-14
func TestDetectCWE22PathSink_PrecisionAndLanguageRecipes(t *testing.T) {
	cases := []struct {
		name       string
		relFile    string
		src        string
		wantBreach bool
		wantSubstr string
	}{
		{
			name:       "go_dynamic_join_breach_emits_go_recipe",
			relFile:    "pkg/store/reader.go",
			src:        "package store\nimport (\"os\"; \"path/filepath\")\nfunc ReadItem(baseDir, relPath string) ([]byte, error) {\n\treturn os.ReadFile(filepath.Join(baseDir, relPath))\n}\n",
			wantBreach: true,
			wantSubstr: "filepath.Rel(absBase, absTarget)",
		},
		{
			name:       "ts_dynamic_join_breach_emits_js_recipe",
			relFile:    "src/store/reader.ts",
			src:        "import fs from 'fs'; import path from 'path';\nexport function readItem(baseDir: string, rel: string) {\n  return fs.readFileSync(path.join(baseDir, rel), 'utf8');\n}\n",
			wantBreach: true,
			wantSubstr: "path.relative(root, target)",
		},
		{
			name:       "go_constant_subpath_passes_cleanly",
			relFile:    "substrate/cli/hooks.go",
			src:        "package cli\nimport (\"os\"; \"path/filepath\")\nconst claudeDirToken = \".claude\"\nfunc load(baseDir string) ([]byte, error) {\n\tp := filepath.Join(baseDir, claudeDirToken, \"settings.json\")\n\treturn os.ReadFile(p)\n}\n",
			wantBreach: false,
		},
		{
			name:       "go_filepath_separator_confinement_passes",
			relFile:    "pkg/store/safe.go",
			src:        "package store\nimport (\"os\"; \"path/filepath\"; \"strings\")\nfunc ReadSafe(baseDir, rel string) ([]byte, error) {\n\tp := filepath.Join(baseDir, rel)\n\tif !strings.HasPrefix(p, baseDir+string(filepath.Separator)) { return nil, nil }\n\treturn os.ReadFile(p)\n}\n",
			wantBreach: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := DetectPolyglotCWESinks(tc.relFile, []byte(tc.src))
			if tc.wantBreach && (len(hits) == 0 || !strings.Contains(hits[0], tc.wantSubstr)) {
				t.Fatalf("expected CWE-22 hit containing %q, got %v", tc.wantSubstr, hits)
			}
			if !tc.wantBreach && len(hits) != 0 {
				t.Fatalf("expected zero CWE-22 hits, got %v", hits)
			}
		})
	}
}
