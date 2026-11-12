// Package polyrun implements polyglot toolchain syntax and test runner auto-discovery.
package polyrun

import (
	"context"
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// EvaluatePolyglotToolchains executes deterministic polyrun verification logic (@intent nomos_intent_traceability_spec.md).
func EvaluatePolyglotToolchains(ctx *vscope.VerifyContext, root string) ([]string, *vscope.GateResult) {
	cfg := vscope.ResolveVerifyConfig(ctx)
	if cfg != nil && !cfg.Governance.QualityGates.IsPolyglotToolchainsEnabled() {
		return nil, nil
	}

	scopeFiles := resolveToolchainScopeFiles(ctx, root)
	type polyProbe struct {
		impact func(string) bool
		run    func() (string, *vscope.GateResult)
	}
	probes := []polyProbe{
		{isPythonImpactPath, func() (string, *vscope.GateResult) { return verifyPythonToolchain(ctx, root) }},
		{isRubyImpactPath, func() (string, *vscope.GateResult) { return verifyRubyToolchain(ctx, root) }},
		{isTypeScriptImpactPath, func() (string, *vscope.GateResult) { return VerifyJSAndTypeScriptToolchain(ctx, root) }},
		{isRustImpactPath, func() (string, *vscope.GateResult) { return verifyRustToolchain(root) }},
		{isCCppImpactPath, func() (string, *vscope.GateResult) { return verifyCCppBuildAndSyntax(ctx, root) }},
		{isJVMImpactPath, func() (string, *vscope.GateResult) { return verifyJVMToolchain(root) }},
		{isCSharpImpactPath, func() (string, *vscope.GateResult) { return verifyCSharpToolchain(root) }},
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
	if astdebt.IsJSFamilyExt(filepath.Ext(lower)) {
		return true
	}
	base := filepath.Base(lower)
	return base == "tsconfig.json" || base == "package.json"
}

func isRustImpactPath(lower string) bool {
	base := filepath.Base(lower)
	return strings.HasSuffix(lower, ".rs") || base == "cargo.toml" || base == "cargo.lock"
}

func verifyPythonToolchain(ctx *vscope.VerifyContext, root string) (string, *vscope.GateResult) {
	pyBin, err := ResolvePythonBinary(root, "python3")
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
	cmd.Env = SanitizedToolchainEnv(EnvPyNoBytecodeKV, EnvToolchainGateKV)
	if out, cErr := cmd.CombinedOutput(); cErr != nil {
		return "", ToolchainFailure("Python syntax compilation failed", string(out), "fix Python syntax errors (python3 -m py_compile)")
	}

	if HasPythonTests(root) {
		targets, shouldRun := ResolveScopedPytestTargets(ctx, root)
		if shouldRun {
			if pytestBin, pErr := ResolvePythonBinary(root, "pytest"); pErr == nil {
				pytestArgs := append([]string{"-q", "--tb=short", "-p", "no:cacheprovider"}, targets...)
				tCmd := exec.CommandContext(cmdCtx, pytestBin, pytestArgs...)
				tCmd.Dir = root
				tCmd.Env = SanitizedToolchainEnv(EnvPyNoBytecodeKV, EnvToolchainGateKV)
				if out, tErr := tCmd.CombinedOutput(); tErr != nil {
					return "", ToolchainFailure("Python pytest suite failed", string(out), "resolve failing Python unit tests (pytest -q)")
				}
				return "Python(compile+pytest)", nil
			}
			return RunPythonUnittestFallback(cmdCtx, pyBin, root, targets)
		}
	}
	return "Python(compile)", nil
}

func collectTargetFilesByExt(ctx *vscope.VerifyContext, root, ext string) []string {
	if files := collectModifiedFilesByExt(ctx, ext); len(files) > 0 {
		return files
	}
	return walkRepoFilesByExt(root, ext)
}

func collectModifiedFilesByExt(ctx *vscope.VerifyContext, ext string) []string {
	modified, err := vscope.GetModifiedPolyglotFiles(ctx)
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
			if impact.IsSkippableSecurityDir(d.Name()) && path != root {
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

// HasPythonTests executes deterministic polyrun verification logic (@intent nomos_intent_traceability_spec.md).
func HasPythonTests(root string) bool {
	found := false
	if walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || found {
			return nil
		}
		if d.IsDir() {
			if impact.IsSkippableSecurityDir(d.Name()) && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if (strings.HasPrefix(name, astdebt.PrefixPyTest) && strings.HasSuffix(name, astdebt.ExtPy)) || strings.HasSuffix(name, astdebt.SuffixPyTest) {
			found = true
		}
		return nil
	}); walkErr != nil {
		return false
	}
	return found
}

func verifyTypeScriptToolchain(root string) (string, *vscope.GateResult) {
	return VerifyJSAndTypeScriptToolchain(nil, root)
}

func verifyRustToolchain(root string) (string, *vscope.GateResult) {
	if _, err := os.Stat(filepath.Join(root, "Cargo.toml")); err != nil {
		return "", nil
	}
	cargoBin, err := exec.LookPath("cargo")
	if err != nil {
		return "", nil
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if HasRustTests(root) {
		tCmd := exec.CommandContext(cmdCtx, cargoBin, astdebt.DirTest, "--no-fail-fast", "--quiet")
		tCmd.Dir = root
		tCmd.Env = SanitizedToolchainEnv(EnvToolchainGateKV)
		if out, tErr := tCmd.CombinedOutput(); tErr != nil {
			return "", ToolchainFailure("Rust cargo test failed", string(out), "resolve failing Rust unit tests or borrow errors (cargo test)")
		}
		return "Rust(cargo+test)", nil
	}
	cmd := exec.CommandContext(cmdCtx, cargoBin, "check", "--quiet")
	cmd.Dir = root
	cmd.Env = SanitizedToolchainEnv(EnvToolchainGateKV)
	if out, cErr := cmd.CombinedOutput(); cErr != nil {
		return "", ToolchainFailure("Rust cargo check failed", string(out), "resolve Rust borrow/type errors (cargo check)")
	}
	return "Rust(cargo)", nil
}

// ToolchainFailure executes deterministic polyrun verification logic (@intent nomos_intent_traceability_spec.md).
func ToolchainFailure(prefix, rawOut, remedy string) *vscope.GateResult {
	msg := extractSalientToolchainWindow(rawOut, 400)
	return &vscope.GateResult{
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
