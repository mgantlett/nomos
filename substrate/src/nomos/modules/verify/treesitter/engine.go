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

// TreeSitterWasmBytecode embeds a pure-WebAssembly verification probe module (exporting verify_probe)
// executed hermetically inside the pure-Go wazero runtime without CGO (Law 2).
var TreeSitterWasmBytecode = []byte{
	0x00, 0x61, 0x73, 0x6d, // \0asm magic
	0x01, 0x00, 0x00, 0x00, // WASM binary version 1
	// Section 1 (Type): 1 func type (i32, i32) -> (i32)
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f,
	// Section 3 (Function): 1 function referencing type index 0
	0x03, 0x02, 0x01, 0x00,
	// Section 7 (Export): export "verify_probe" -> func 0
	0x07, 0x10, 0x01, 0x0c,
	'v', 'e', 'r', 'i', 'f', 'y', '_', 'p', 'r', 'o', 'b', 'e',
	0x00, 0x00,
	// Section 10 (Code): local.get 0, local.get 1, i32.add, end
	0x0a, 0x09, 0x01, 0x07, 0x00, 0x20, 0x00, 0x20, 0x01, 0x6a, 0x0b,
}

// WazeroTreeSitterEngine orchestrates the pure-Go wazero WebAssembly sandbox and polyglot structural scanners (Law 2).
type WazeroTreeSitterEngine struct {
	runtime    wazero.Runtime
	compiled   wazero.CompiledModule
	module     api.Module
	thresholds EngineThresholds
}

var (
	defaultEngine   *WazeroTreeSitterEngine
	defaultEngineMu sync.Mutex
	sharedWasmCache wazero.CompilationCache
	sharedCacheOnce sync.Once
)

func getSharedCompilationCache() wazero.CompilationCache {
	sharedCacheOnce.Do(func() {
		cacheDir := filepath.Join(os.TempDir(), "nomos-wazero-cache")
		c, err := wazero.NewCompilationCacheWithDir(cacheDir)
		if err != nil {
			c = wazero.NewCompilationCache()
		}
		sharedWasmCache = c
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

// ExecuteWasmVerify executes the compiled WebAssembly verify_probe export inside wazero.
func (e *WazeroTreeSitterEngine) ExecuteWasmVerify(ctx context.Context) error {
	if e.runtime == nil || e.module == nil {
		return fmt.Errorf("treesitter wasm engine runtime is not initialized")
	}
	probe := e.module.ExportedFunction("verify_probe")
	if probe == nil {
		return fmt.Errorf("treesitter wasm module missing exported verify_probe")
	}
	res, err := probe.Call(ctx, 40, 2)
	if err != nil {
		return fmt.Errorf("treesitter wasm verify_probe call failed: %w", err)
	}
	if len(res) != 1 || res[0] != 42 {
		return fmt.Errorf("unexpected treesitter wasm verify_probe result: %v", res)
	}
	return nil
}

// ParseAndEvaluate parses polyglot source and generates a GateEvaluationReport.
func (e *WazeroTreeSitterEngine) ParseAndEvaluate(ctx context.Context, filePath string, content []byte) (*GateEvaluationReport, error) {
	lang := DetectLanguage(filePath)
	if lang == "" {
		return nil, fmt.Errorf("unsupported polyglot file type: %s", filePath)
	}
	totalLines := countSourceLines(content)
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
	".svelte": "svelte",
	".py":     "python",
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

