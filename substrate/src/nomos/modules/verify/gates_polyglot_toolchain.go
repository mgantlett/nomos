// Package verify implements polyglot toolchain syntax and test runner auto-discovery.
package verify

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func evaluatePolyglotToolchains(ctx *VerifyContext, root string) ([]string, *GateResult) {
	cfg := resolveVerifyConfig(ctx)
	if cfg != nil && !cfg.Governance.QualityGates.IsPolyglotToolchainsEnabled() {
		return nil, nil
	}

	scopeFiles := resolveToolchainScopeFiles(ctx, root)
	type polyProbe struct {
		impact func(string) bool
		run    func() (string, *GateResult)
	}
	probes := []polyProbe{
		{isPythonImpactPath, func() (string, *GateResult) { return verifyPythonToolchain(ctx, root) }},
		{isRubyImpactPath, func() (string, *GateResult) { return verifyRubyToolchain(ctx, root) }},
		{isTypeScriptImpactPath, func() (string, *GateResult) { return verifyJSAndTypeScriptToolchain(ctx, root) }},
		{isRustImpactPath, func() (string, *GateResult) { return verifyRustToolchain(root) }},
		{isCCppImpactPath, func() (string, *GateResult) { return verifyCCppBuildAndSyntax(ctx, root) }},
		{isJVMImpactPath, func() (string, *GateResult) { return verifyJVMToolchain(root) }},
		{isCSharpImpactPath, func() (string, *GateResult) { return verifyCSharpToolchain(root) }},
	}

	var verified []string
	for _, p := range probes {
		if len(scopeFiles) > 0 && !hasPolyglotStackImpact(scopeFiles, p.impact) {
			continue
		}
		label, fail := p.run()
		if fail != nil {
			return nil, fail
		}
		if label != "" {
			verified = append(verified, label)
		}
	}
	return verified, nil
}

func hasPolyglotStackImpact(files []string, match func(string) bool) bool {
	for _, f := range files {
		if match(strings.ToLower(filepath.ToSlash(f))) {
			return true
		}
	}
	return false
}

func isPythonImpactPath(lower string) bool {
	base := filepath.Base(lower)
	return strings.HasSuffix(lower, ".py") || base == "pyproject.toml" || base == "requirements.txt" || base == "setup.py"
}

func isTypeScriptImpactPath(lower string) bool {
	if isJSFamilyExt(filepath.Ext(lower)) {
		return true
	}
	base := filepath.Base(lower)
	return base == "tsconfig.json" || base == "package.json"
}

func isRustImpactPath(lower string) bool {
	base := filepath.Base(lower)
	return strings.HasSuffix(lower, ".rs") || base == "cargo.toml" || base == "cargo.lock"
}

func verifyPythonToolchain(ctx *VerifyContext, root string) (string, *GateResult) {
	pyBin, err := resolvePythonBinary(root, "python3")
	if err != nil {
		return "", nil
	}
	pyFiles := collectTargetFilesByExt(ctx, root, ".py")
	if len(pyFiles) == 0 {
		return "", nil
	}

	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pyCheckScript := "import sys\nfor p in sys.argv[1:]:\n    with open(p, 'rb') as f:\n        compile(f.read(), p, 'exec')\n"
	args := append([]string{"-B", "-c", pyCheckScript}, pyFiles...)
	cmd := exec.CommandContext(cmdCtx, pyBin, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), envPyNoBytecodeKV, envToolchainGateKV)
	if out, cErr := cmd.CombinedOutput(); cErr != nil {
		return "", toolchainFailure("Python syntax compilation failed", string(out), "fix Python syntax errors (python3 -m py_compile)")
	}

	if hasPythonTests(root) {
		targets, shouldRun := resolveScopedPytestTargets(ctx, root)
		if shouldRun {
			if pytestBin, pErr := resolvePythonBinary(root, "pytest"); pErr == nil {
				pytestArgs := append([]string{"-q", "--tb=short", "-p", "no:cacheprovider"}, targets...)
				tCmd := exec.CommandContext(cmdCtx, pytestBin, pytestArgs...)
				tCmd.Dir = root
				tCmd.Env = append(os.Environ(), envPyNoBytecodeKV, envToolchainGateKV)
				if out, tErr := tCmd.CombinedOutput(); tErr != nil {
					return "", toolchainFailure("Python pytest suite failed", string(out), "resolve failing Python unit tests (pytest -q)")
				}
				return "Python(compile+pytest)", nil
			}
			return runPythonUnittestFallback(cmdCtx, pyBin, root, targets)
		}
	}
	return "Python(compile)", nil
}

func collectTargetFilesByExt(ctx *VerifyContext, root, ext string) []string {
	if files := collectModifiedFilesByExt(ctx, ext); len(files) > 0 {
		return files
	}
	return walkRepoFilesByExt(root, ext)
}

func collectModifiedFilesByExt(ctx *VerifyContext, ext string) []string {
	modified, err := getModifiedPolyglotFiles(ctx)
	if err != nil || len(modified) == 0 {
		return nil
	}
	var files []string
	for p := range modified {
		if !strings.HasSuffix(p, ext) {
			continue
		}
		if fi, sErr := os.Stat(p); sErr == nil && !fi.IsDir() {
			files = append(files, p)
		}
	}
	return files
}

func walkRepoFilesByExt(root, ext string) []string {
	var files []string
	if walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if isSkippableSecurityDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ext) && len(files) < 50 {
			files = append(files, path)
		}
		return nil
	}); walkErr != nil {
		return nil
	}
	return files
}

func hasPythonTests(root string) bool {
	found := false
	if walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || found {
			return nil
		}
		if d.IsDir() {
			if isSkippableSecurityDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if (strings.HasPrefix(name, prefixPyTest) && strings.HasSuffix(name, extPy)) || strings.HasSuffix(name, suffixPyTest) {
			found = true
		}
		return nil
	}); walkErr != nil {
		return false
	}
	return found
}

func verifyTypeScriptToolchain(root string) (string, *GateResult) {
	return verifyJSAndTypeScriptToolchain(nil, root)
}

func verifyRustToolchain(root string) (string, *GateResult) {
	if _, err := os.Stat(filepath.Join(root, "Cargo.toml")); err != nil {
		return "", nil
	}
	cargoBin, err := exec.LookPath("cargo")
	if err != nil {
		return "", nil
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if hasRustTests(root) {
		tCmd := exec.CommandContext(cmdCtx, cargoBin, dirTest, "--no-fail-fast", "--quiet")
		tCmd.Dir = root
		tCmd.Env = append(os.Environ(), envToolchainGateKV)
		if out, tErr := tCmd.CombinedOutput(); tErr != nil {
			return "", toolchainFailure("Rust cargo test failed", string(out), "resolve failing Rust unit tests or borrow errors (cargo test)")
		}
		return "Rust(cargo+test)", nil
	}
	cmd := exec.CommandContext(cmdCtx, cargoBin, "check", "--quiet")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), envToolchainGateKV)
	if out, cErr := cmd.CombinedOutput(); cErr != nil {
		return "", toolchainFailure("Rust cargo check failed", string(out), "resolve Rust borrow/type errors (cargo check)")
	}
	return "Rust(cargo)", nil
}

func toolchainFailure(prefix, rawOut, remedy string) *GateResult {
	msg := extractSalientToolchainWindow(rawOut, 400)
	return &GateResult{
		GateName: "Go Toolchain Gate",
		Passed:   false,
		Message:  fmt.Sprintf("%s: %s", prefix, msg),
		Remedy:   remedy,
	}
}

func extractSalientToolchainWindow(rawOut string, maxLen int) string {
	msg := strings.TrimSpace(rawOut)
	if len(msg) <= maxLen {
		return msg
	}
	for _, marker := range []string{"\nnot ok ", "\nFAIL ", "\nFAIL\t", "AssertionError", "SyntaxError", "TypeError", "Error:"} {
		if idx := strings.Index(msg, marker); idx >= 0 {
			window := strings.TrimSpace(msg[idx:])
			if len(window) > maxLen {
				return window[:maxLen] + "..."
			}
			return window
		}
	}
	return msg[:maxLen] + "..."
}
