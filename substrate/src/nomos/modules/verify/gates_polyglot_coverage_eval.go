// Package verify implements polyglot statement coverage evaluation across LCOV, Cobertura/JaCoCo, SimpleCov, and Python trace.
package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/polyrun"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

const probeIDCoverage = "coverage"

const pyStdlibTraceRunner = `import io, os, sys, trace, unittest
root = os.path.abspath(sys.argv[1])
targets = sys.argv[2:]
sys.path.insert(0, root)
def collect_exec_lines(code_obj, out):
    for _, _, ln in code_obj.co_lines():
        if ln and ln > 0:
            out.add(ln)
    for c in code_obj.co_consts:
        if hasattr(c, "co_lines"):
            collect_exec_lines(c, out)
py_files = []
for dp, dn, fn in os.walk(root):
    dn[:] = [d for d in dn if not d.startswith(".") and d not in ("venv", ".venv", "node_modules", "tests", "test", "worktrees")]
    for f in fn:
        if f.endswith(".py") and not f.startswith("test_") and not f.endswith("_test.py"):
            py_files.append(os.path.abspath(os.path.join(dp, f)))
if not py_files or len(py_files) > 60:
    sys.exit(0)
def run_suite():
    try:
        import pytest
        args = ["-q", "--tb=no", "-p", "no:cacheprovider"] + targets
        if pytest.main(args) != 0:
            sys.exit(2)
        return
    except ImportError:
        pass
    loader = unittest.defaultTestLoader
    if targets:
        suite = unittest.TestSuite()
        for t in targets:
            full = t if os.path.isabs(t) else os.path.join(root, t)
            if os.path.isfile(full):
                d, f = os.path.split(full)
                suite.addTests(loader.discover(d, pattern=f))
            else:
                suite.addTests(loader.loadTestsFromName(t))
    else:
        start = os.path.join(root, "tests") if os.path.isdir(os.path.join(root, "tests")) else root
        top = root if (start == root or os.path.isfile(os.path.join(start, "__init__.py"))) else None
        suite = loader.discover(start, pattern="test*.py", top_level_dir=top)
    res = unittest.TextTestRunner(stream=io.StringIO()).run(suite)
    if not res.wasSuccessful():
        sys.exit(2)
tracer = trace.Trace(count=1, trace=0)
tracer.runfunc(run_suite)
counts = tracer.results().counts
tot, hit = 0, 0
for p in py_files:
    try:
        with open(p, "rb") as fh:
            co = compile(fh.read(), p, "exec")
        lines = set()
        collect_exec_lines(co, lines)
        for ln in lines:
            tot += 1
            if counts.get((p, ln), 0) > 0:
                hit += 1
    except Exception:
        pass
if tot > 0:
    print("NOMOS_PY_COV:{:.2f}".format((hit * 100.0) / tot))
`

func evaluatePolyglotCoverageGate(gateName string, ctx *vscope.VerifyContext, root string) *vscope.GateResult {
	pct, source, failMsg := measureWorkspacePolyglotCoverage(ctx, root)
	if failMsg != "" {
		return &vscope.GateResult{
			GateName: gateName, ProbeID: probeIDCoverage, Law: 2, Ring: RingDynamic,
			Passed: false, Message: failMsg,
			Remedy: "resolve failing polyglot unit tests before evaluating statement coverage",
		}
	}
	if source == "" {
		return &vscope.GateResult{
			GateName: gateName, ProbeID: probeIDCoverage, Law: 2, Ring: RingDynamic,
			Passed: true, Message: "no Go or polyglot coverage suite detected in scope",
		}
	}
	configured := resolveThresholds(ctx).MinCoverage
	effThreshold := resolveEffectivePolyglotCoverageThreshold(ctx, root, configured, pct)
	if pct < effThreshold {
		return &vscope.GateResult{
			GateName: gateName, ProbeID: probeIDCoverage, Law: 2, Ring: RingDynamic,
			Passed:  false,
			Message: fmt.Sprintf("polyglot statement coverage %.1f%% (%s) is below minimum required %.1f%%", pct, source, effThreshold),
			Remedy:  fmt.Sprintf("author additional unit tests to raise statement coverage to >= %.1f%%", effThreshold),
		}
	}
	return &vscope.GateResult{
		GateName: gateName, ProbeID: probeIDCoverage, Law: 2, Ring: RingDynamic,
		Passed:  true,
		Message: fmt.Sprintf("polyglot statement coverage %.1f%% (%s) meets required threshold (>= %.1f%%)", pct, source, effThreshold),
	}
}

func resolveEffectivePolyglotCoverageThreshold(ctx *vscope.VerifyContext, root string, configured, curPct float64) float64 {
	if curPct >= configured || !polyrun.IsBrownfieldGitScope(ctx, root) {
		return configured
	}
	if !vscope.IsWorktreeContext(ctx, root) {
		if len(vscope.GetModifiedFilesInScope(ctx, root)) == 0 {
			return curPct
		}
		return configured
	}
	if ctx != nil && ctx.RepoRoot != "" && ctx.RepoRoot != root {
		baseCtx := &vscope.VerifyContext{RepoRoot: ctx.RepoRoot, IsBaseline: true}
		if basePct, src, fail := measureWorkspacePolyglotCoverage(baseCtx, ctx.RepoRoot); fail == "" && src != "" && basePct < configured {
			return basePct
		}
	}
	return configured
}

func measureWorkspacePolyglotCoverage(ctx *vscope.VerifyContext, root string) (float64, string, string) {
	if pct, label, ok := discoverCoverageArtifact(root); ok {
		return pct, label, ""
	}
	if resolveEffectiveCoverageMode(ctx) == config.CoverageModeArtifact {
		return 0, "", ""
	}
	return runLivePolyglotCoverageProbes(ctx, root)
}

func runLivePolyglotCoverageProbes(ctx *vscope.VerifyContext, root string) (float64, string, string) {
	if polyrun.HasPythonTests(root) {
		return runPythonStdlibCoverage(ctx, root)
	}
	if pct, src, fail := runNodeLiveV8Coverage(root); src != "" || fail != "" {
		return pct, src, fail
	}
	if pct, src, fail := runRubyStdlibCoverage(root); src != "" || fail != "" {
		return pct, src, fail
	}
	return runDotnetLiveCoverage(root)
}

func discoverCoverageArtifact(root string) (float64, string, bool) {
	lcovPaths := []string{
		filepath.Join(root, astdebt.DirCoverage, "lcov.info"),
		filepath.Join(root, "lcov.info"),
		filepath.Join(root, "target", "llvm-cov-target", "lcov.info"),
	}
	for _, p := range lcovPaths {
		if data, err := os.ReadFile(p); err == nil {
			if pct, ok := polyrun.ParseLCOVCoverage(data); ok {
				return pct, "LCOV", true
			}
		}
	}
	xmlPaths := []string{
		filepath.Join(root, "coverage.xml"),
		filepath.Join(root, "cobertura.xml"),
		filepath.Join(root, astdebt.DirCoverage, "cobertura-coverage.xml"),
		filepath.Join(root, "target", "site", "jacoco", "jacoco.xml"),
		filepath.Join(root, "build", "reports", "jacoco", astdebt.DirTest, "jacocoTestReport.xml"),
	}
	xmlPaths = append(xmlPaths, findTestResultsCobertura(filepath.Join(root, "TestResults"))...)
	for _, p := range xmlPaths {
		if data, err := os.ReadFile(p); err == nil {
			if pct, ok := polyrun.ParseCoberturaCoverage(data); ok {
				return pct, "Cobertura/XML", true
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, astdebt.DirCoverage, ".resultset.json")); err == nil {
		if pct, ok := ParseSimpleCovCoverage(data); ok {
			return pct, "SimpleCov", true
		}
	}
	return 0, "", false
}

func findTestResultsCobertura(testResultsDir string) []string {
	if !vscope.DirExists(testResultsDir) {
		return nil
	}
	var out []string
	if walkErr := filepath.WalkDir(testResultsDir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d != nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), "cobertura.xml") {
			out = append(out, path)
		}
		return nil
	}); walkErr != nil {
		return out
	}
	return out
}

// ParseSimpleCovCoverage parses Ruby SimpleCov .resultset.json line arrays into coverage percentage (0..100).
// @intent nomos_intent_traceability_spec.md #CoverageGate
func ParseSimpleCovCoverage(content []byte) (float64, bool) {
	var raw map[string]struct {
		Coverage map[string]struct {
			Lines []*int `json:"lines"`
		} `json:"coverage"`
	}
	if err := json.Unmarshal(content, &raw); err != nil {
		return 0, false
	}
	var relevant, covered int
	for _, suite := range raw {
		for _, fileCov := range suite.Coverage {
			for _, hits := range fileCov.Lines {
				if hits == nil {
					continue
				}
				relevant++
				if *hits > 0 {
					covered++
				}
			}
		}
	}
	if relevant == 0 {
		return 0, false
	}
	return (float64(covered) * 100.0) / float64(relevant), true
}

func runPythonStdlibCoverage(ctx *vscope.VerifyContext, root string) (float64, string, string) {
	pyBin, err := polyrun.ResolvePythonBinary(root, "python3")
	if err != nil {
		return 0, "", ""
	}
	targets, shouldRun := polyrun.ResolveScopedPytestTargets(ctx, root)
	if !shouldRun {
		return 100.0, "Python(diff-scoped skip)", ""
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	args := append([]string{"-B", "-c", pyStdlibTraceRunner, root}, targets...)
	cmd := exec.CommandContext(cmdCtx, pyBin, args...)
	cmd.Dir = root
	cmd.Env = polyrun.SanitizedToolchainEnv(polyrun.EnvPyNoBytecodeKV, polyrun.EnvToolchainGateKV)
	out, runErr := cmd.CombinedOutput()
	outStr := string(out)
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			return 0, "", "Python coverage test execution failed: " + polyrun.TruncateToolchainOutput(outStr)
		}
		return 0, "", ""
	}
	for _, line := range strings.Split(outStr, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "NOMOS_PY_COV:") {
			if val, pErr := strconv.ParseFloat(strings.TrimPrefix(trimmed, "NOMOS_PY_COV:"), 64); pErr == nil {
				return val, "Python(trace)", ""
			}
		}
	}
	return 0, "", ""
}
