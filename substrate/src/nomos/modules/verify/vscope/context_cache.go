// Package vscope provides scoped per-pipeline memoization for configuration and git file discovery.
package vscope

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sync"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

// ParsedGoEntry holds a cached parsed Go AST alongside its token.FileSet.
type ParsedGoEntry struct {
	Fset *token.FileSet
	File *ast.File
	Err  error
}

type readBytesEntry struct {
	data []byte
	err  error
}

type walkFilesEntry struct {
	paths []string
	err   error
}

// verifyContextMemo stores read-only workspace snapshot state for the duration of a single Pipeline.Execute pass.
// Keeping this cache external to VerifyContext preserves exact struct field parity with compiled @intent specs.
type verifyContextMemo struct {
	mu               sync.Mutex
	cfgResolved      bool
	cfg              *config.EcosystemConfig
	polyglotResolved bool
	polyglotFiles    map[string]bool
	polyglotErr      error
	slicesByKey      map[string][]string
	renameMaps       map[string]map[string]string
	parsedGoByKey    map[string]ParsedGoEntry
	readBytesByPath  map[string]readBytesEntry
	walkFilesByKey   map[string]walkFilesEntry
	keyLocks         map[string]*sync.Mutex
}

var activeVerifyContextCache sync.Map

// beginVerifyContextCache enables read-only memoization for ctx across a Pipeline.Execute pass.
func BeginVerifyContextCache(ctx *VerifyContext) {
	if ctx == nil {
		return
	}
	activeVerifyContextCache.Store(ctx, &verifyContextMemo{
		slicesByKey:     make(map[string][]string),
		renameMaps:      make(map[string]map[string]string),
		parsedGoByKey:   make(map[string]ParsedGoEntry),
		readBytesByPath: make(map[string]readBytesEntry),
		walkFilesByKey:  make(map[string]walkFilesEntry),
		keyLocks:        make(map[string]*sync.Mutex),
	})
}

// endVerifyContextCache evicts the memoized snapshot for ctx after Pipeline.Execute completes.
func EndVerifyContextCache(ctx *VerifyContext) {
	if ctx == nil {
		return
	}
	activeVerifyContextCache.Delete(ctx)
}

func lookupVerifyContextMemo(ctx *VerifyContext) *verifyContextMemo {
	if ctx == nil {
		return nil
	}
	raw, ok := activeVerifyContextCache.Load(ctx)
	if !ok {
		return nil
	}
	memo, castOK := raw.(*verifyContextMemo)
	if !castOK {
		return nil
	}
	return memo
}

// MemoizedVerifyConfig executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func MemoizedVerifyConfig(ctx *VerifyContext, loader func() *config.EcosystemConfig) *config.EcosystemConfig {
	memo := lookupVerifyContextMemo(ctx)
	if memo == nil {
		return loader()
	}
	memo.mu.Lock()
	if memo.cfgResolved {
		cached := memo.cfg
		memo.mu.Unlock()
		return cached
	}
	memo.mu.Unlock()

	loaded := loader()

	memo.mu.Lock()
	memo.cfg = loaded
	memo.cfgResolved = true
	memo.mu.Unlock()
	return loaded
}

func memoizedPolyglotFiles(ctx *VerifyContext, loader func() (map[string]bool, error)) (map[string]bool, error) {
	memo := lookupVerifyContextMemo(ctx)
	if memo == nil {
		return loader()
	}
	memo.mu.Lock()
	if memo.polyglotResolved {
		files, err := memo.polyglotFiles, memo.polyglotErr
		memo.mu.Unlock()
		return files, err
	}
	memo.mu.Unlock()

	files, err := loader()

	memo.mu.Lock()
	memo.polyglotFiles = files
	memo.polyglotErr = err
	memo.polyglotResolved = true
	memo.mu.Unlock()
	return files, err
}

// MemoizedScopeFiles executes deterministic vscope verification logic (@intent nomos_intent_traceability_spec.md).
func MemoizedScopeFiles(ctx *VerifyContext, workDir string, loader func() []string) []string {
	return memoizedStringSlice(ctx, "scope:"+workDir, loader)
}

func memoizedScopeExcludes(ctx *VerifyContext, key string, loader func() []string) []string {
	return memoizedStringSlice(ctx, "excludes:"+key, loader)
}

func memoizedStringSlice(ctx *VerifyContext, fullKey string, loader func() []string) []string {
	return memoizedKeyedValue(ctx, func(m *verifyContextMemo) map[string][]string { return m.slicesByKey }, fullKey, loader)
}

// MemoizedRenameMap caches git rename mappings per gitRoot and baseRef for ctx (@intent intent.md tree_sitter_ast_gate_spec.md).
func MemoizedRenameMap(ctx *VerifyContext, key string, loader func() map[string]string) map[string]string {
	return memoizedKeyedValue(ctx, func(m *verifyContextMemo) map[string]map[string]string { return m.renameMaps }, key, loader)
}

func memoizedKeyedValue[V any](ctx *VerifyContext, selectMap func(*verifyContextMemo) map[string]V, key string, loader func() V) V {
	memo := lookupVerifyContextMemo(ctx)
	if memo == nil {
		return loader()
	}
	memo.mu.Lock()
	target := selectMap(memo)
	if cached, ok := target[key]; ok {
		memo.mu.Unlock()
		return cached
	}
	memo.mu.Unlock()

	loaded := loader()

	memo.mu.Lock()
	selectMap(memo)[key] = loaded
	memo.mu.Unlock()
	return loaded
}

// MemoizedParseGoFile parses and caches a Go source file AST and FileSet within an active VerifyContext cache.
func MemoizedParseGoFile(ctx *VerifyContext, path string, mode parser.Mode) (*token.FileSet, *ast.File, error) {
	key := fmt.Sprintf("%d:%s", mode, path)
	entry := memoizedKeyedValue(ctx, func(m *verifyContextMemo) map[string]ParsedGoEntry { return m.parsedGoByKey }, key, func() ParsedGoEntry {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, nil, mode)
		return ParsedGoEntry{Fset: fset, File: node, Err: err}
	})
	return entry.Fset, entry.File, entry.Err
}

// MemoizedReadFile reads and caches file bytes for path within an active VerifyContext cache.
func MemoizedReadFile(ctx *VerifyContext, path string) ([]byte, error) {
	entry := memoizedKeyedValue(ctx, func(m *verifyContextMemo) map[string]readBytesEntry { return m.readBytesByPath }, path, func() readBytesEntry {
		data, err := os.ReadFile(path)
		return readBytesEntry{data: data, err: err}
	})
	return entry.data, entry.err
}

// MemoizedWalkFiles caches directory walk file slices per key within an active VerifyContext cache.
func MemoizedWalkFiles(ctx *VerifyContext, key string, loader func() ([]string, error)) ([]string, error) {
	entry := memoizedKeyedValue(ctx, func(m *verifyContextMemo) map[string]walkFilesEntry { return m.walkFilesByKey }, key, func() walkFilesEntry {
		paths, err := loader()
		return walkFilesEntry{paths: paths, err: err}
	})
	return entry.paths, entry.err
}

// LockContextKey acquires a per-context keyed mutex for the duration of a Pipeline.Execute pass.
func LockContextKey(ctx *VerifyContext, key string) func() {
	memo := lookupVerifyContextMemo(ctx)
	if memo == nil {
		return func() {}
	}
	memo.mu.Lock()
	kmu, ok := memo.keyLocks[key]
	if !ok {
		kmu = &sync.Mutex{}
		memo.keyLocks[key] = kmu
	}
	memo.mu.Unlock()
	kmu.Lock()
	return kmu.Unlock
}
