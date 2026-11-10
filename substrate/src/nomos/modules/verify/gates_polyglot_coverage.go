package verify

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func isCCppImpactPath(lower string) bool {
	if isCFamilyExt(filepath.Ext(lower)) {
		return true
	}
	return filepath.Base(lower) == "cmakelists.txt"
}

func isJVMImpactPath(lower string) bool {
	if isJVMFamilyExt(filepath.Ext(lower)) {
		return true
	}
	base := filepath.Base(lower)
	return base == "build.gradle" || base == "build.gradle.kts" || base == "pom.xml"
}

func isCSharpImpactPath(lower string) bool {
	ext := filepath.Ext(lower)
	return ext == extCS || ext == ".csproj" || ext == ".sln"
}

func isRubyImpactPath(lower string) bool {
	ext := filepath.Ext(lower)
	if ext == extRB || ext == extRake {
		return true
	}
	base := filepath.Base(lower)
	return base == "gemfile" || base == "gemfile.lock" || base == "rakefile"
}

var minitestSkipSummaryRe = regexp.MustCompile(`(?i)\b([1-9][0-9]*)\s+skips?\b`)

// verifyRubyToolchain compiles Ruby sources via InstructionSequence and runs Rails/Rake Minitest suites when present.
func verifyRubyToolchain(ctx *VerifyContext, root string) (string, *GateResult) {
	rubyBin, err := exec.LookPath("ruby")
	if err != nil {
		return "", nil
	}
	rbFiles := collectTargetFilesByExt(ctx, root, ".rb")
	if len(rbFiles) == 0 {
		return "", nil
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rbCheckScript := "ARGV.each { |p| RubyVM::InstructionSequence.compile_file(p) }"
	args := append([]string{"-e", rbCheckScript}, rbFiles...)
	cmd := exec.CommandContext(cmdCtx, rubyBin, args...)
	cmd.Dir = root
	cmd.Env = sanitizedToolchainEnv("NOMOS_IN_TOOLCHAIN_GATE=1")
	if out, cErr := cmd.CombinedOutput(); cErr != nil {
		return "", toolchainFailure("Ruby syntax compilation failed", string(out), "fix Ruby syntax errors (ruby -c)")
	}

	if hasRubyMinitestSuite(root) {
		if bin, mArgs, ok := resolveRubyMinitestCommand(root, rubyBin); ok {
			return executeRubyMinitestRunner(root, bin, mArgs)
		}
	}
	return "Ruby(compile)", nil
}

// hasRubyMinitestSuite reports whether root/test contains any *_test.rb Minitest files.
func hasRubyMinitestSuite(root string) bool {
	testDir := filepath.Join(root, dirTest)
	fi, err := os.Stat(testDir)
	if err != nil || !fi.IsDir() {
		return false
	}
	found := false
	if walkErr := filepath.WalkDir(testDir, func(path string, d fs.DirEntry, wErr error) error {
		if wErr != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if isSkippableSecurityDir(d.Name()) && path != testDir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.rb") {
			found = true
			return fs.SkipAll
		}
		return nil
	}); walkErr != nil {
		return found
	}
	return found
}

// resolveRubyMinitestCommand selects bin/rails test or bundle exec rake test for a Rails/Minitest workspace.
func resolveRubyMinitestCommand(root, rubyBin string) (string, []string, bool) {
	railsBinstub := filepath.Join(root, "bin", "rails")
	if fi, err := os.Stat(railsBinstub); err == nil && !fi.IsDir() {
		if fi.Mode().Perm()&0111 != 0 {
			return railsBinstub, []string{dirTest}, true
		}
		return rubyBin, []string{filepath.Join("bin", "rails"), dirTest}, true
	}
	if !fileExistsInDir(root, "Rakefile") {
		return "", nil, false
	}
	if fileExistsInDir(root, "Gemfile") {
		if bundleBin, bErr := exec.LookPath("bundle"); bErr == nil {
			return bundleBin, []string{"exec", "rake", dirTest}, true
		}
	}
	if rakeBin, rErr := exec.LookPath("rake"); rErr == nil {
		return rakeBin, []string{dirTest}, true
	}
	return "", nil, false
}

// executeRubyMinitestRunner runs the discovered Minitest command and rejects assertion failures or skipped tests.
func executeRubyMinitestRunner(root, bin string, args []string) (string, *GateResult) {
	tCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tCmd := exec.CommandContext(tCtx, bin, args...)
	tCmd.Dir = root
	tCmd.Env = sanitizedToolchainEnv("RAILS_ENV=test", "NOMOS_IN_TOOLCHAIN_GATE=1")
	out, err := tCmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return "", toolchainFailure("Ruby Minitest suite failed", output, "resolve failing Minitest assertions (bin/rails test)")
	}
	if hasSkippedMinitestOutput(output) {
		return "", toolchainFailure("Ruby Minitest suite contains skipped tests", output, "implement or remove skipped Minitest cases (bin/rails test)")
	}
	return "Ruby(compile+minitest)", nil
}

// hasSkippedMinitestOutput detects non-zero skip counts or verbose Skipped sections in Minitest output.
func hasSkippedMinitestOutput(output string) bool {
	if minitestSkipSummaryRe.MatchString(output) {
		return true
	}
	return strings.Contains(output, "\nSkipped:\n")
}

func verifyCCppToolchain(root string) (string, *GateResult) {
	return verifyCCppBuildAndSyntax(nil, root)
}

func verifyJVMToolchain(root string) (string, *GateResult) {
	hasGradle := fileExistsInDir(root, "build.gradle") || fileExistsInDir(root, "build.gradle.kts")
	hasMaven := fileExistsInDir(root, "pom.xml")
	if !hasGradle && !hasMaven {
		return "", nil
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	runTests := hasJVMTests(root)

	if hasGradle {
		gradleBin := filepath.Join(root, "gradlew")
		if !fileExists(gradleBin) {
			var err error
			if gradleBin, err = exec.LookPath("gradle"); err != nil {
				return "", nil
			}
		}
		goal, label := "testClasses", "JVM(gradle)"
		if runTests {
			goal, label = dirTest, "JVM(gradle+test)"
		}
		cmd := exec.CommandContext(cmdCtx, gradleBin, goal, "--quiet")
		cmd.Dir = root
		cmd.Env = sanitizedToolchainEnv(envToolchainGateKV)
		if out, gErr := cmd.CombinedOutput(); gErr != nil {
			return "", toolchainFailure("JVM gradle "+goal+" failed", string(out), "resolve Java/Kotlin compiler or test errors")
		}
		return label, nil
	}
	mvnBin := filepath.Join(root, "mvnw")
	if !fileExists(mvnBin) {
		var err error
		if mvnBin, err = exec.LookPath("mvn"); err != nil {
			return "", nil
		}
	}
	goal, label := "test-compile", "JVM(maven)"
	if runTests {
		goal, label = dirTest, "JVM(maven+test)"
	}
	cmd := exec.CommandContext(cmdCtx, mvnBin, goal, "-q")
	cmd.Dir = root
	cmd.Env = sanitizedToolchainEnv(envToolchainGateKV)
	if out, mErr := cmd.CombinedOutput(); mErr != nil {
		return "", toolchainFailure("JVM maven "+goal+" failed", string(out), "resolve Java/Kotlin compiler or test errors")
	}
	return label, nil
}

func verifyCSharpToolchain(root string) (string, *GateResult) {
	projects, hasTests := findCSharpProjects(root)
	if len(projects) == 0 {
		return "", nil
	}
	dotnetBin, err := exec.LookPath("dotnet")
	if err != nil {
		return "", nil
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	subcmd, label := "build", "C#(dotnet)"
	if hasTests {
		subcmd, label = dirTest, "C#(dotnet+test)"
	}
	cmd := exec.CommandContext(cmdCtx, dotnetBin, subcmd, "--nologo", "-v", "q")
	cmd.Dir = root
	cmd.Env = sanitizedToolchainEnv(envToolchainGateKV)
	if out, dErr := cmd.CombinedOutput(); dErr != nil {
		return "", toolchainFailure("C# dotnet "+subcmd+" failed", string(out), "resolve C# compiler or unit test errors (dotnet "+subcmd+")")
	}
	return label, nil
}

func fileExistsInDir(dir, name string) bool {
	fi, err := os.Stat(filepath.Join(dir, name))
	return err == nil && !fi.IsDir()
}

var coberturaLineRateRe = regexp.MustCompile(`line-rate=["']([0-9.]+)["']`)

// ParseLCOVCoverage parses LCOV (LF:/LH:) line coverage records and returns percentage (0..100).
func ParseLCOVCoverage(content []byte) (float64, bool) {
	var totalLF, totalLH int
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "LF:") {
			if v, err := strconv.Atoi(strings.TrimPrefix(trimmed, "LF:")); err == nil {
				totalLF += v
			}
		} else if strings.HasPrefix(trimmed, "LH:") {
			if v, err := strconv.Atoi(strings.TrimPrefix(trimmed, "LH:")); err == nil {
				totalLH += v
			}
		}
	}
	if totalLF <= 0 {
		return 0, false
	}
	return (float64(totalLH) * 100.0) / float64(totalLF), true
}

// ParseCoberturaCoverage parses Cobertura/JaCoCo XML line-rate attribute and returns percentage (0..100).
func ParseCoberturaCoverage(content []byte) (float64, bool) {
	m := coberturaLineRateRe.FindSubmatch(content)
	if len(m) < 2 {
		return 0, false
	}
	rate, err := strconv.ParseFloat(string(m[1]), 64)
	if err != nil || rate < 0 {
		return 0, false
	}
	if rate <= 1.0 {
		return rate * 100.0, true
	}
	return rate, true
}
