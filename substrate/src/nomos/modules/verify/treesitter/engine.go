package treesitter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WazeroTreeSitterEngine orchestrates the pure-Go wazero WebAssembly sandbox and polyglot structural scanners (Law 2).
type WazeroTreeSitterEngine struct {
	runtime    wazero.Runtime
	compiled   wazero.CompiledModule
	module     api.Module
	wasmMu     sync.Mutex
	thresholds EngineThresholds
}

var (
	defaultEngine   *WazeroTreeSitterEngine
	defaultEngineMu sync.Mutex
	sharedWasmCache wazero.CompilationCache
	sharedCacheOnce sync.Once
)

func resolvePrivateWazeroCacheDir() string {
	baseDir, err := os.UserCacheDir()
	if err != nil || baseDir == "" {
		baseDir = filepath.Join(os.TempDir(), fmt.Sprintf("nomos-cache-%d", os.Getuid()))
	}
	target := filepath.Join(baseDir, "nomos", "wazero")
	if mkErr := os.MkdirAll(target, 0700); mkErr != nil {
		return ""
	}
	fi, statErr := os.Lstat(target)
	if statErr != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	return target
}

func getSharedCompilationCache() wazero.CompilationCache {
	sharedCacheOnce.Do(func() {
		cacheDir := resolvePrivateWazeroCacheDir()
		if cacheDir != "" {
			if c, err := wazero.NewCompilationCacheWithDir(cacheDir); err == nil {
				sharedWasmCache = c
				return
			}
		}
		sharedWasmCache = wazero.NewCompilationCache()
	})
	return sharedWasmCache
}

// GetDefaultEngine retrieves or initializes the canonical shared WazeroTreeSitterEngine.
func GetDefaultEngine() (*WazeroTreeSitterEngine, error) {
	defaultEngineMu.Lock()
	defer defaultEngineMu.Unlock()
	if defaultEngine != nil {
		return defaultEngine, nil
	}
	eng, err := NewTreeSitterEngine(context.Background())
	if err != nil {
		return nil, err
	}
	defaultEngine = eng
	return defaultEngine, nil
}

// NewTreeSitterEngine initializes a pure-Go WebAssembly runtime with a persistent CompilationCache and verifies query contracts.
func NewTreeSitterEngine(ctx context.Context) (*WazeroTreeSitterEngine, error) {
	if err := ValidateQueryFS(); err != nil {
		return nil, fmt.Errorf("failed validating query filesystem: %w", err)
	}

	cfg := wazero.NewRuntimeConfig()
	if cache := getSharedCompilationCache(); cache != nil {
		cfg = cfg.WithCompilationCache(cache)
	}
	rt := wazero.NewRuntimeWithConfig(ctx, cfg)

	compiled, err := rt.CompileModule(ctx, TreeSitterWasmBytecode)
	if err != nil {
		if cErr := rt.Close(ctx); cErr != nil {
			return nil, fmt.Errorf("failed compiling treesitter wasm bytecode (%v): %w", cErr, err)
		}
		return nil, fmt.Errorf("failed compiling treesitter wasm bytecode: %w", err)
	}

	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("treesitter_wasm"))
	if err != nil {
		if cErr := rt.Close(ctx); cErr != nil {
			return nil, fmt.Errorf("failed instantiating treesitter wasm module (%v): %w", cErr, err)
		}
		return nil, fmt.Errorf("failed instantiating treesitter wasm module: %w", err)
	}

	return &WazeroTreeSitterEngine{
		runtime:    rt,
		compiled:   compiled,
		module:     mod,
		thresholds: DefaultEngineThresholds(),
	}, nil
}

// Close terminates the wazero runtime and frees WebAssembly host memory.
func (e *WazeroTreeSitterEngine) Close(ctx context.Context) error {
	if e.runtime != nil {
		return e.runtime.Close(ctx)
	}
	return nil
}

// Runtime returns the underlying wazero WebAssembly runtime instance.
func (e *WazeroTreeSitterEngine) Runtime() wazero.Runtime {
	return e.runtime
}

// CompiledModule returns the pre-compiled Tree-sitter WebAssembly module.
func (e *WazeroTreeSitterEngine) CompiledModule() wazero.CompiledModule {
	return e.compiled
}

// Module returns the active instantiated Tree-sitter WebAssembly module.
func (e *WazeroTreeSitterEngine) Module() api.Module {
	return e.module
}

// ParseAndEvaluate parses polyglot source and generates a GateEvaluationReport.
func (e *WazeroTreeSitterEngine) ParseAndEvaluate(ctx context.Context, filePath string, content []byte) (*GateEvaluationReport, error) {
	lang := DetectLanguage(filePath)
	if lang == "" {
		return nil, fmt.Errorf("unsupported polyglot file type: %s", filePath)
	}
	totalLines, wasmErr := e.scanLinesWasm(ctx, content)
	if wasmErr != nil {
		totalLines = countSourceLines(content)
	}
	if rep, handled, err := evaluateNonFunctionLanguage(lang, filePath, content, totalLines); handled {
		return rep, err
	}
	funcs, err := ExtractFunctions(lang, filePath, content)
	if err != nil {
		return nil, err
	}
	rep := BuildEvaluationReport(filePath, totalLines, funcs, e.thresholds)
	if skipped := DetectSkippedTests(lang, content); len(skipped) > 0 {
		rep.SkippedTests = skipped
		rep.Passed = false
	}
	return rep, nil
}

func evaluateNonFunctionLanguage(lang, filePath string, content []byte, totalLines int) (*GateEvaluationReport, bool, error) {
	switch lang {
	case "markdown":
		levels, verified := scanMarkdown(filePath, content)
		rep := emptyFuncReport(filePath, totalLines)
		rep.HeadingLevels = levels
		rep.OutlineVerified = verified
		return rep, true, nil
	case "yaml":
		return emptyFuncReport(filePath, totalLines), true, nil
	case "html":
		if err := ValidateHTMLSyntax(filePath, content); err != nil {
			return nil, true, err
		}
		return emptyFuncReport(filePath, totalLines), true, nil
	case "css":
		if err := ValidateBraceSyntax("css", filePath, content); err != nil {
			return nil, true, err
		}
		return emptyFuncReport(filePath, totalLines), true, nil
	default:
		return nil, false, nil
	}
}

func emptyFuncReport(filePath string, totalLines int) *GateEvaluationReport {
	return &GateEvaluationReport{
		Functions: []FunctionMetric{},
		Files:     []FileMetric{{Name: filePath, LOC: totalLines, CDS: 0}},
		Density:   100,
		Passed:    true,
	}
}

var extToLangMap = map[string]string{
	".go":     "go",
	".ts":     "typescript",
	".tsx":    "typescript",
	".js":     "typescript",
	".jsx":    "typescript",
	".mjs":    "typescript",
	".cjs":    "typescript",
	".svelte": "svelte",
	".py":     "python",
	".rb":     "ruby",
	".rake":   "ruby",
	".rs":     "rust",
	".c":      "c",
	".h":      "c",
	".cpp":    "cpp",
	".cc":     "cpp",
	".cxx":    "cpp",
	".hpp":    "cpp",
	".java":   "java",
	".kt":     "kotlin",
	".kts":    "kotlin",
	".cs":     "csharp",
	".html":   "html",
	".htm":    "html",
	".css":    "css",
	".md":     "markdown",
	".yaml":   "yaml",
	".yml":    "yaml",
}

// DetectLanguage resolves the canonical language identifier from the file extension.
func DetectLanguage(filePath string) string {
	return extToLangMap[strings.ToLower(filepath.Ext(filePath))]
}

// BuildEvaluationReport calculates docstring density, CDS, and gate verdicts from extracted functions.
func BuildEvaluationReport(filePath string, totalLines int, funcs []FunctionMetric, thresh EngineThresholds) *GateEvaluationReport {
	docCount := 0
	var emptyBlocks []string
	for _, fn := range funcs {
		if fn.Documented {
			docCount++
		}
		if fn.EmptyBlock {
			emptyBlocks = append(emptyBlocks, fn.Name)
		}
	}

	density := 100
	if len(funcs) > 0 {
		density = (docCount * 100) / len(funcs)
	}

	cds := CalculateCDS(funcs, density)
	passed := true

	if cds > thresh.MaxCDS || totalLines > thresh.MaxFileLOC || len(emptyBlocks) > 0 {
		passed = false
	}
	for _, fn := range funcs {
		if fn.Complexity > thresh.MaxComplexity || fn.LOC > thresh.MaxFuncLOC {
			passed = false
			break
		}
	}

	return &GateEvaluationReport{
		Functions:   funcs,
		Files:       []FileMetric{{Name: filePath, LOC: totalLines, CDS: cds}},
		Documented:  docCount,
		Total:       len(funcs),
		Density:     density,
		EmptyBlocks: emptyBlocks,
		Passed:      passed,
	}
}

func countSourceLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}
	lines := 1
	for _, b := range content {
		if b == '\n' {
			lines++
		}
	}
	return lines
}
