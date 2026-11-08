// Package verify implements polyglot test runners and syntax compilers across JS/TS, Python, Rust, C/C++, JVM, and C#.
package verify

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	envToolchainGateKV = "NOMOS_IN_TOOLCHAIN_GATE=1"
	envPyNoBytecodeKV  = "PYTHONDONTWRITEBYTECODE=1"
)

const pyUnittestRunnerScript = `import os, sys, unittest
root = os.path.abspath(sys.argv[1])
targets = [os.path.relpath(t, root) if os.path.isabs(t) else t for t in sys.argv[2:]]
sys.path.insert(0, root)
loader = unittest.defaultTestLoader
if targets:
    suite = unittest.TestSuite()
    for t in targets:
        if os.path.isfile(os.path.join(root, t)):
            d, f = os.path.split(os.path.join(root, t))
            suite.addTests(loader.discover(d, pattern=f))
        else:
            suite.addTests(loader.loadTestsFromName(t))
else:
    cand = os.path.join(root, "tests")
    if not os.path.isdir(cand):
        cand = os.path.join(root, "test") if os.path.isdir(os.path.join(root, "test")) else root
    top = root if (cand == root or os.path.isfile(os.path.join(cand, "__init__.py"))) else None
    suite = loader.discover(cand, pattern="test*.py", top_level_dir=top)
res = unittest.TextTestRunner(verbosity=0).run(suite)
sys.exit(0 if res.wasSuccessful() else 1)
`

func runPythonUnittestFallback(cmdCtx context.Context, pyBin, root string, targets []string) (string, *GateResult) {
	args := append([]string{"-B", "-c", pyUnittestRunnerScript, root}, targets...)
	tCmd := exec.CommandContext(cmdCtx, pyBin, args...)
	tCmd.Dir = root
	tCmd.Env = append(os.Environ(), envPyNoBytecodeKV, envToolchainGateKV)
	if out, err := tCmd.CombinedOutput(); err != nil {
		return "", toolchainFailure("Python unittest suite failed", string(out), "resolve failing Python unit tests (python3 -m unittest)")
	}
	return "Python(compile+unittest)", nil
}

func verifyJSAndTypeScriptToolchain(ctx *VerifyContext, root string) (string, *GateResult) {
	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	jsChecked, jsFail := verifyPureJSSyntax(cmdCtx, ctx, root)
	if jsFail != nil {
		return "", jsFail
	}
	tsChecked, tsFail := verifyTSCompiler(cmdCtx, root)
	if tsFail != nil {
		return "", tsFail
	}
	tested, testFail := runJSTestSuite(cmdCtx, root)
	if testFail != nil {
		return "", testFail
	}
	return formatJSToolchainLabel(tsChecked, jsChecked, tested), nil
}

func formatJSToolchainLabel(tsChecked, jsChecked, tested bool) string {
	if tsChecked && tested {
		return "TypeScript(tsc+test)"
	}
	if tsChecked {
		return "TypeScript(tsc)"
	}
	if jsChecked && tested {
		return "JavaScript(node+test)"
	}
	if tested {
		return "JavaScript(test)"
	}
	if jsChecked {
		return "JavaScript(node)"
	}
	return ""
}

func verifyPureJSSyntax(cmdCtx context.Context, ctx *VerifyContext, root string) (bool, *GateResult) {
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		return false, nil
	}
	var jsFiles []string
	for _, ext := range []string{extJS, extMJS, extCJS} {
		jsFiles = append(jsFiles, collectTargetFilesByExt(ctx, root, ext)...)
	}
	checked := false
	for _, p := range limitSlice(jsFiles, 25) {
		data, rErr := os.ReadFile(p)
		if rErr != nil || looksLikeJSXSource(string(data)) {
			continue
		}
		cmd := exec.CommandContext(cmdCtx, nodeBin, "--check", p)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), envToolchainGateKV)
		if out, cErr := cmd.CombinedOutput(); cErr != nil {
			return false, toolchainFailure("JavaScript syntax check failed", string(out), "fix JavaScript syntax errors (node --check)")
		}
		checked = true
	}
	return checked, nil
}

func looksLikeJSXSource(src string) bool {
	return strings.Contains(src, "</") && (strings.Contains(src, "react") || strings.Contains(src, "jsx"))
}

func verifyTSCompiler(cmdCtx context.Context, root string) (bool, *GateResult) {
	if !fileExistsInDir(root, "tsconfig.json") {
		return false, nil
	}
	tscBin := filepath.Join(root, "node_modules", ".bin", "tsc")
	if !fileExists(tscBin) {
		return false, nil
	}
	cmd := exec.CommandContext(cmdCtx, tscBin, "--noEmit")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), envToolchainGateKV)
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, toolchainFailure("TypeScript typecheck failed", string(out), "resolve TypeScript compiler errors (tsc --noEmit)")
	}
	return true, nil
}

func runJSTestSuite(cmdCtx context.Context, root string) (bool, *GateResult) {
	testFiles := collectJSTestFiles(root)
	if len(testFiles) == 0 {
		return false, nil
	}
	bin, args, ok := resolveJSTestCommand(root, testFiles)
	if !ok {
		return false, nil
	}
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CI=true", envToolchainGateKV)
	if out, err := cmd.CombinedOutput(); err != nil {
		return false, toolchainFailure("JavaScript/TypeScript test suite failed", string(out), "resolve failing JS/TS unit tests")
	}
	return true, nil
}

func resolveJSTestCommand(root string, testFiles []string) (string, []string, bool) {
	binDir := filepath.Join(root, "node_modules", ".bin")
	if vitest := filepath.Join(binDir, "vitest"); fileExists(vitest) {
		return vitest, []string{"run"}, true
	}
	if jest := filepath.Join(binDir, "jest"); fileExists(jest) {
		return jest, []string{"--ci"}, true
	}
	if mocha := filepath.Join(binDir, "mocha"); fileExists(mocha) {
		return mocha, nil, true
	}
	if usesNodeBuiltinTest(testFiles) {
		if nodeBin, err := exec.LookPath("node"); err == nil {
			return nodeBin, append([]string{"--test"}, testFiles...), true
		}
	}
	if canRunNpmTestScript(root) {
		if npmBin, err := exec.LookPath("npm"); err == nil {
			return npmBin, []string{dirTest, "--silent"}, true
		}
	}
	return "", nil, false
}

func collectJSTestFiles(root string) []string {
	var tests []string
	if walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if path != root && isSkippableSecurityDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".d.ts") && isJSTestOrDeclFile(name) && len(tests) < 40 {
			tests = append(tests, path)
		}
		return nil
	}); walkErr != nil {
		return tests
	}
	return tests
}

func usesNodeBuiltinTest(testFiles []string) bool {
	for _, p := range limitSlice(testFiles, 10) {
		data, err := os.ReadFile(p)
		if err == nil && strings.Contains(string(data), "node:test") {
			return true
		}
	}
	return false
}

func canRunNpmTestScript(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if jErr := json.Unmarshal(data, &pkg); jErr != nil {
		return false
	}
	script := strings.TrimSpace(pkg.Scripts[dirTest])
	if script == "" || strings.Contains(script, "no test specified") {
		return false
	}
	if strings.HasPrefix(script, "node ") {
		return true
	}
	return dirExists(filepath.Join(root, "node_modules"))
}
