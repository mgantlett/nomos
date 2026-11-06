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
		{isTypeScriptImpactPath, func() (string, *GateResult) { return verifyTypeScriptToolchain(root) }},
		{isRustImpactPath, func() (string, *GateResult) { return verifyRustToolchain(root) }},
		{isCCppImpactPath, func() (string, *GateResult) { return verifyCCppToolchain(root) }},
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
	switch filepath.Ext(lower) {
	case ".ts", ".tsx", ".js", ".jsx", ".svelte":
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
	pyBin, err := exec.LookPath("python3")
	if err != nil {
		return "", nil
	}
	pyFiles := collectTargetFilesByExt(ctx, root, ".py")
	if len(pyFiles) == 0 {
		return "", nil
	}

	cmdCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pyCheckScript := "import sys\nfor p in sys.argv[1:]:\n    with open(p, 'rb') as f:\n        compile(f.read(), p, 'exec')\n"
	args := append([]string{"-B", "-c", pyCheckScript}, pyFiles...)
	cmd := exec.CommandContext(cmdCtx, pyBin, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1", "NOMOS_IN_TOOLCHAIN_GATE=1")
	if out, cErr := cmd.CombinedOutput(); cErr != nil {
		return "", toolchainFailure("Python syntax compilation failed", string(out), "fix Python syntax errors (python3 -m py_compile)")
	}

	if hasPythonTests(root) {
		if pytestBin, pErr := exec.LookPath("pytest"); pErr == nil {
			tCmd := exec.CommandContext(cmdCtx, pytestBin, "-q", "--tb=short", "-p", "no:cacheprovider")
			tCmd.Dir = root
			tCmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1", "NOMOS_IN_TOOLCHAIN_GATE=1")
			if out, tErr := tCmd.CombinedOutput(); tErr != nil {
				return "", toolchainFailure("Python pytest suite failed", string(out), "resolve failing Python unit tests (pytest -q)")
			}
			return "Python(compile+pytest)", nil
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
		if (strings.HasPrefix(name, "test_") && strings.HasSuffix(name, ".py")) || strings.HasSuffix(name, "_test.py") {
			found = true
		}
		return nil
	}); walkErr != nil {
		return false
	}
	return found
}

func verifyTypeScriptToolchain(root string) (string, *GateResult) {
	tsconfig := filepath.Join(root, "tsconfig.json")
	tscBin := filepath.Join(root, "node_modules", ".bin", "tsc")
	if _, err := os.Stat(tsconfig); err != nil {
		return "", nil
	}
	if _, err := os.Stat(tscBin); err != nil {
		return "", nil
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, tscBin, "--noEmit")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NOMOS_IN_TOOLCHAIN_GATE=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", toolchainFailure("TypeScript typecheck failed", string(out), "resolve TypeScript compiler errors (tsc --noEmit)")
	}
	return "TypeScript(tsc)", nil
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

	cmd := exec.CommandContext(cmdCtx, cargoBin, "check", "--quiet")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NOMOS_IN_TOOLCHAIN_GATE=1")
	if out, cErr := cmd.CombinedOutput(); cErr != nil {
		return "", toolchainFailure("Rust cargo check failed", string(out), "resolve Rust borrow/type errors (cargo check)")
	}
	return "Rust(cargo)", nil
}

func toolchainFailure(prefix, rawOut, remedy string) *GateResult {
	msg := strings.TrimSpace(rawOut)
	if len(msg) > 400 {
		msg = msg[:400] + "..."
	}
	return &GateResult{
		GateName: "Go Toolchain Gate",
		Passed:   false,
		Message:  fmt.Sprintf("%s: %s", prefix, msg),
		Remedy:   remedy,
	}
}
