// Package verify implements zero-dependency live coverage probes for Node V8, Ruby stdlib Coverage, and .NET XPlat.
package verify

import (
	"context"
	"encoding/json"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/polyrun"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const rubyStdlibCoverageScript = `require "coverage"
Coverage.start(lines: true)
root = File.expand_path(ARGV.shift)
$LOAD_PATH.unshift(root, File.join(root, "lib"), File.join(root, "test"))
at_exit do
  st = $!
  if st.nil? || (st.is_a?(SystemExit) && st.success?)
    res = Coverage.result
    tot = 0
    hit = 0
    res.each do |file, data|
      f = File.expand_path(file)
      next unless f.start_with?(root + "/")
      base = File.basename(f)
      next if base.end_with?("_test.rb", "_spec.rb") || f.include?("/test/") || f.include?("/spec/") || f.include?("/vendor/")
      lines = data.is_a?(Hash) ? data[:lines] : data
      next unless lines
      lines.each do |cnt|
        next if cnt.nil?
        tot += 1
        hit += 1 if cnt > 0
      end
    end
    puts format("NOMOS_RB_COV:%.2f", (hit * 100.0) / tot) if tot > 0
  end
end
ARGV.each { |tf| load File.expand_path(tf) }
`

type v8CoverageFile struct {
	Result []v8ScriptCov `json:"result"`
}

type v8ScriptCov struct {
	URL       string      `json:"url"`
	Functions []v8FuncCov `json:"functions"`
}

type v8FuncCov struct {
	Ranges []v8RangeCov `json:"ranges"`
}

type v8RangeCov struct {
	StartOffset int `json:"startOffset"`
	EndOffset   int `json:"endOffset"`
	Count       int `json:"count"`
}

func runNodeLiveV8Coverage(root string) (float64, string, string) {
	testFiles := polyrun.CollectJSTestFiles(root)
	if len(testFiles) == 0 {
		return 0, "", ""
	}
	bin, args, ok := polyrun.ResolveJSTestCommand(root, testFiles)
	if !ok {
		return 0, "", ""
	}
	covDir, err := os.MkdirTemp("", "nomos-v8cov-*")
	if err != nil {
		return 0, "", ""
	}
	defer func() {
		if rmErr := os.RemoveAll(covDir); rmErr != nil {
			return
		}
	}()

	cmdCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, bin, args...)
	cmd.Dir = root
	cmd.Env = polyrun.SanitizedToolchainEnv("CI=true", "NODE_V8_COVERAGE="+covDir, polyrun.EnvToolchainGateKV)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return 0, "", "JS/TS coverage test execution failed: " + polyrun.TruncateToolchainOutput(string(out))
	}
	pct, parsed := parseNodeV8CoverageDir(root, covDir)
	if !parsed {
		return 0, "", ""
	}
	return pct, "JavaScript/TS(V8)", ""
}

func parseNodeV8CoverageDir(root, covDir string) (float64, bool) {
	entries, err := os.ReadDir(covDir)
	if err != nil {
		return 0, false
	}
	absRoot, rErr := filepath.Abs(root)
	if rErr != nil {
		absRoot = root
	}
	scriptRanges := make(map[string][]v8RangeCov)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(covDir, e.Name()))
		if readErr != nil {
			continue
		}
		collectV8ScriptRanges(absRoot, raw, scriptRanges)
	}
	var totalLines, hitLines int
	for scriptPath, ranges := range scriptRanges {
		tot, hit := computeV8FileLineCoverage(scriptPath, ranges)
		totalLines += tot
		hitLines += hit
	}
	if totalLines == 0 {
		return 0, false
	}
	return (float64(hitLines) * 100.0) / float64(totalLines), true
}

func collectV8ScriptRanges(absRoot string, raw []byte, out map[string][]v8RangeCov) {
	var doc v8CoverageFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		return
	}
	for _, s := range doc.Result {
		scriptPath := resolveV8FileURL(s.URL)
		if scriptPath == "" || !strings.HasPrefix(scriptPath, absRoot+string(filepath.Separator)) {
			continue
		}
		if vscope.IsExcludedPolyglotDir(filepath.ToSlash(scriptPath)) || !isPolyglotImplFileForTDD(filepath.Base(scriptPath)) {
			continue
		}
		for _, fn := range s.Functions {
			out[scriptPath] = append(out[scriptPath], fn.Ranges...)
		}
	}
}

func resolveV8FileURL(rawURL string) string {
	if !strings.HasPrefix(rawURL, "file://") {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return filepath.FromSlash(u.Path)
}

func computeV8FileLineCoverage(scriptPath string, ranges []v8RangeCov) (int, int) {
	src, err := os.ReadFile(scriptPath)
	if err != nil || len(src) == 0 {
		return 0, 0
	}
	byteCovered := evaluateV8ByteHits(len(src), ranges)
	lines := strings.Split(string(src), "\n")
	offset, tot, hit := 0, 0, 0
	for _, line := range lines {
		lineLen := len(line)
		trimmed := strings.TrimSpace(line)
		if isCountableJSLine(trimmed) && offset < len(byteCovered) {
			tot++
			if isLineRangeHit(byteCovered, offset, offset+lineLen) {
				hit++
			}
		}
		offset += lineLen + 1
	}
	return tot, hit
}

func evaluateV8ByteHits(srcLen int, ranges []v8RangeCov) []bool {
	hits := make([]bool, srcLen)
	widths := make([]int, srcLen)
	for i := range widths {
		widths[i] = srcLen + 1
	}
	for _, r := range ranges {
		start := maxInt(0, r.StartOffset)
		end := astdebt.MinInt(srcLen, r.EndOffset)
		span := end - start
		if span <= 0 {
			continue
		}
		for idx := start; idx < end; idx++ {
			if span < widths[idx] || (span == widths[idx] && r.Count > 0) {
				widths[idx] = span
				hits[idx] = r.Count > 0
			}
		}
	}
	return hits
}

func isCountableJSLine(trimmed string) bool {
	if trimmed == "" || trimmed == "{" || trimmed == "}" || trimmed == "};" || trimmed == ");" {
		return false
	}
	return !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") && !strings.HasPrefix(trimmed, "*")
}

func isLineRangeHit(byteCovered []bool, start, end int) bool {
	limit := astdebt.MinInt(len(byteCovered), end)
	for i := start; i < limit; i++ {
		if byteCovered[i] {
			return true
		}
	}
	return false
}

func runRubyStdlibCoverage(root string) (float64, string, string) {
	rubyBin, err := exec.LookPath("ruby")
	if err != nil || polyrun.FileExistsInDir(root, "Gemfile.lock") {
		return 0, "", ""
	}
	testFiles := collectRubyUnitTestFiles(root)
	if len(testFiles) == 0 {
		return 0, "", ""
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := append([]string{"-e", rubyStdlibCoverageScript, root}, testFiles...)
	cmd := exec.CommandContext(cmdCtx, rubyBin, args...)
	cmd.Dir = root
	cmd.Env = polyrun.SanitizedToolchainEnv(polyrun.EnvToolchainGateKV)
	out, runErr := cmd.CombinedOutput()
	outStr := string(out)
	if runErr != nil {
		return 0, "", ""
	}
	for _, line := range strings.Split(outStr, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "NOMOS_RB_COV:") {
			if val, pErr := strconv.ParseFloat(strings.TrimPrefix(trimmed, "NOMOS_RB_COV:"), 64); pErr == nil {
				return val, "Ruby(Coverage)", ""
			}
		}
	}
	return 0, "", ""
}

func collectRubyUnitTestFiles(root string) []string {
	var out []string
	for _, relDir := range []string{astdebt.DirTest, "."} {
		dir := filepath.Join(root, relDir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), "_test.rb") {
				out = append(out, filepath.Join(dir, e.Name()))
			}
		}
	}
	return out
}

func runDotnetLiveCoverage(root string) (float64, string, string) {
	projects, hasSDK := polyrun.FindCSharpProjects(root)
	if len(projects) == 0 || !hasSDK {
		return 0, "", ""
	}
	dotnetBin, err := exec.LookPath("dotnet")
	if err != nil {
		return 0, "", ""
	}
	resDir, tErr := os.MkdirTemp("", "nomos-dotnetcov-*")
	if tErr != nil {
		return 0, "", ""
	}
	defer func() {
		if rmErr := os.RemoveAll(resDir); rmErr != nil {
			return
		}
	}()
	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, dotnetBin, astdebt.DirTest, "--nologo", "-v", "q", "--collect:XPlat Code Coverage", "--results-directory", resDir)
	cmd.Dir = root
	cmd.Env = polyrun.SanitizedToolchainEnv(polyrun.EnvToolchainGateKV)
	if _, runErr := cmd.CombinedOutput(); runErr != nil {
		return 0, "", ""
	}
	for _, xmlPath := range findTestResultsCobertura(resDir) {
		if data, rErr := os.ReadFile(xmlPath); rErr == nil {
			if pct, ok := polyrun.ParseCoberturaCoverage(data); ok {
				return pct, "C#(XPlat)", ""
			}
		}
	}
	return 0, "", ""
}
