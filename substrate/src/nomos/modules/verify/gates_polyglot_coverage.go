package verify

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func isCCppImpactPath(lower string) bool {
	switch filepath.Ext(lower) {
	case ".c", ".h", ".cpp", ".cc", ".cxx", ".hpp":
		return true
	}
	return filepath.Base(lower) == "cmakelists.txt"
}

func isJVMImpactPath(lower string) bool {
	switch filepath.Ext(lower) {
	case ".java", ".kt", ".kts":
		return true
	}
	base := filepath.Base(lower)
	return base == "build.gradle" || base == "build.gradle.kts" || base == "pom.xml"
}

func isCSharpImpactPath(lower string) bool {
	ext := filepath.Ext(lower)
	return ext == ".cs" || ext == ".csproj" || ext == ".sln"
}

func verifyCCppToolchain(root string) (string, *GateResult) {
	if _, err := os.Stat(filepath.Join(root, "CMakeLists.txt")); err != nil {
		return "", nil
	}
	cmakeBin, err := exec.LookPath("cmake")
	if err != nil {
		return "", nil
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, cmakeBin, "--version")
	cmd.Dir = root
	if out, cErr := cmd.CombinedOutput(); cErr != nil {
		return "", toolchainFailure("C/C++ cmake probe failed", string(out), "verify CMakeLists.txt and cmake installation")
	}
	return "C/C++(cmake)", nil
}

func verifyJVMToolchain(root string) (string, *GateResult) {
	hasGradle := fileExistsInDir(root, "build.gradle") || fileExistsInDir(root, "build.gradle.kts")
	hasMaven := fileExistsInDir(root, "pom.xml")
	if !hasGradle && !hasMaven {
		return "", nil
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if hasGradle {
		gradleBin, err := exec.LookPath("gradle")
		if err != nil {
			return "", nil
		}
		cmd := exec.CommandContext(cmdCtx, gradleBin, "testClasses", "--quiet")
		cmd.Dir = root
		if out, gErr := cmd.CombinedOutput(); gErr != nil {
			return "", toolchainFailure("JVM gradle testClasses failed", string(out), "resolve Java/Kotlin compilation errors (gradle testClasses)")
		}
		return "JVM(gradle)", nil
	}
	mvnBin, err := exec.LookPath("mvn")
	if err != nil {
		return "", nil
	}
	cmd := exec.CommandContext(cmdCtx, mvnBin, "test-compile", "-q")
	cmd.Dir = root
	if out, mErr := cmd.CombinedOutput(); mErr != nil {
		return "", toolchainFailure("JVM maven test-compile failed", string(out), "resolve Java/Kotlin compilation errors (mvn test-compile)")
	}
	return "JVM(maven)", nil
}

func verifyCSharpToolchain(root string) (string, *GateResult) {
	matches, err := filepath.Glob(filepath.Join(root, "*.csproj"))
	if err != nil || len(matches) == 0 {
		return "", nil
	}
	dotnetBin, err := exec.LookPath("dotnet")
	if err != nil {
		return "", nil
	}
	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, dotnetBin, "build", "--nologo", "-v", "q")
	cmd.Dir = root
	if out, dErr := cmd.CombinedOutput(); dErr != nil {
		return "", toolchainFailure("C# dotnet build failed", string(out), "resolve C# compiler errors (dotnet build)")
	}
	return "C#(dotnet)", nil
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
