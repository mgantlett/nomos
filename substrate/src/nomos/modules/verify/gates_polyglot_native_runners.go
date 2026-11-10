// Package verify implements native Rust, C/C++, JVM, and C# compilation and test execution probes.
package verify

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func hasRustTests(root string) bool {
	if dirExists(filepath.Join(root, "tests")) {
		return true
	}
	found := false
	if walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || found {
			return nil
		}
		if d.IsDir() {
			if path != root && isSkippableSecurityDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".rs") && hasInlineRustTestModule(path) {
			found = true
		}
		return nil
	}); walkErr != nil {
		return found
	}
	return found
}

func verifyCCppBuildAndSyntax(ctx *VerifyContext, root string) (string, *GateResult) {
	cmdCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if fileExistsInDir(root, "CMakeLists.txt") {
		if label, fail, handled := runCMakeToolchain(cmdCtx, root); handled || fail != nil {
			return label, fail
		}
	}
	return verifyStandaloneCCppSyntax(cmdCtx, ctx, root)
}

func runCMakeToolchain(cmdCtx context.Context, root string) (string, *GateResult, bool) {
	cmakeBin, err := exec.LookPath("cmake")
	if err != nil {
		return "", nil, false
	}
	rawCMake, rErr := os.ReadFile(filepath.Join(root, "CMakeLists.txt"))
	if rErr != nil || !strings.Contains(strings.ToLower(string(rawCMake)), "project(") {
		cmd := exec.CommandContext(cmdCtx, cmakeBin, "--version")
		cmd.Dir = root
		if out, cErr := cmd.CombinedOutput(); cErr != nil {
			return "", toolchainFailure("C/C++ cmake probe failed", string(out), "verify CMakeLists.txt and cmake installation"), true
		}
		return "C/C++(cmake)", nil, true
	}
	buildDir, cleanup := resolveCMakeBuildDir(root)
	if cleanup != nil {
		defer cleanup()
	}
	if !fileExistsInDir(buildDir, "CMakeCache.txt") {
		cfgCmd := exec.CommandContext(cmdCtx, cmakeBin, "-S", root, "-B", buildDir)
		cfgCmd.Env = sanitizedToolchainEnv(envToolchainGateKV)
		if out, cfgErr := cfgCmd.CombinedOutput(); cfgErr != nil {
			return "", toolchainFailure("C/C++ cmake configure failed", string(out), "fix CMakeLists.txt configuration errors"), true
		}
	}
	bCmd := exec.CommandContext(cmdCtx, cmakeBin, "--build", buildDir)
	bCmd.Env = sanitizedToolchainEnv(envToolchainGateKV)
	if out, bErr := bCmd.CombinedOutput(); bErr != nil {
		return "", toolchainFailure("C/C++ cmake build failed", string(out), "resolve C/C++ compiler errors (cmake --build)"), true
	}
	if fileExistsInDir(buildDir, "CTestTestfile.cmake") {
		if ctestBin, tErr := exec.LookPath("ctest"); tErr == nil {
			tCmd := exec.CommandContext(cmdCtx, ctestBin, "--test-dir", buildDir, "--output-on-failure")
			tCmd.Env = sanitizedToolchainEnv(envToolchainGateKV)
			if out, runErr := tCmd.CombinedOutput(); runErr != nil {
				return "", toolchainFailure("C/C++ ctest suite failed", string(out), "resolve failing CTest unit tests (ctest --output-on-failure)"), true
			}
			return "C/C++(cmake+ctest)", nil, true
		}
	}
	return "C/C++(cmake+build)", nil, true
}

func resolveCMakeBuildDir(root string) (string, func()) {
	existing := filepath.Join(root, "build")
	if fileExistsInDir(existing, "CMakeCache.txt") {
		return existing, nil
	}
	tmp, err := os.MkdirTemp("", "nomos-cmake-*")
	if err != nil {
		return existing, nil
	}
	return tmp, func() {
		if rmErr := os.RemoveAll(tmp); rmErr != nil {
			return
		}
	}
}

func verifyStandaloneCCppSyntax(cmdCtx context.Context, ctx *VerifyContext, root string) (string, *GateResult) {
	cFiles := collectTargetFilesByExt(ctx, root, extC)
	var cppFiles []string
	for _, ext := range []string{extCPP, extCC, extCXX} {
		cppFiles = append(cppFiles, collectTargetFilesByExt(ctx, root, ext)...)
	}
	checked := false
	if len(cFiles) > 0 {
		if ccBin := firstAvailableBinary("cc", "gcc", "clang"); ccBin != "" {
			if fail := runSyntaxCompiler(cmdCtx, ccBin, nil, root, cFiles); fail != nil {
				return "", fail
			}
			checked = true
		}
	}
	if len(cppFiles) > 0 {
		if cxxBin := firstAvailableBinary("c++", "g++", "clang++"); cxxBin != "" {
			if fail := runSyntaxCompiler(cmdCtx, cxxBin, []string{"-std=c++17"}, root, cppFiles); fail != nil {
				return "", fail
			}
			checked = true
		}
	}
	if checked {
		return "C/C++(syntax)", nil
	}
	return "", nil
}

func runSyntaxCompiler(cmdCtx context.Context, bin string, extraFlags []string, root string, files []string) *GateResult {
	for _, f := range limitSlice(files, 20) {
		args := append([]string{"-fsyntax-only", "-I" + root, "-I" + filepath.Dir(f)}, extraFlags...)
		args = append(args, f)
		cmd := exec.CommandContext(cmdCtx, bin, args...)
		cmd.Dir = root
		cmd.Env = sanitizedToolchainEnv(envToolchainGateKV)
		if out, err := cmd.CombinedOutput(); err != nil {
			return toolchainFailure("C/C++ syntax compilation failed", string(out), "fix C/C++ compiler syntax errors (-fsyntax-only)")
		}
	}
	return nil
}

func firstAvailableBinary(names ...string) string {
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return ""
}

func hasJVMTests(root string) bool {
	if dirExists(filepath.Join(root, "src", dirTest)) {
		return true
	}
	found := false
	if walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || found {
			return nil
		}
		if d.IsDir() {
			if path != root && isSkippableSecurityDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "Test.java") || strings.HasSuffix(d.Name(), "Test.kt") {
			found = true
		}
		return nil
	}); walkErr != nil {
		return found
	}
	return found
}

func findCSharpProjects(root string) ([]string, bool) {
	var projects []string
	hasTestSDK := false
	if walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			rel, rErr := filepath.Rel(root, path)
			if path != root && (isSkippableSecurityDir(d.Name()) || (rErr == nil && strings.Count(filepath.ToSlash(rel), "/") > 2)) {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext == ".csproj" || ext == ".sln" {
			projects = append(projects, path)
			if ext == ".csproj" && hasCSharpTestReference(path) {
				hasTestSDK = true
			}
		}
		return nil
	}); walkErr != nil {
		return projects, hasTestSDK
	}
	return projects, hasTestSDK
}

func hasCSharpTestReference(csprojPath string) bool {
	data, err := os.ReadFile(csprojPath)
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft.net.test.sdk") || strings.Contains(lower, "xunit") ||
		strings.Contains(lower, "nunit") || strings.Contains(lower, "mstest")
}
