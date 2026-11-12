// Package vscope provides scoped per-pipeline memoization for configuration and git file discovery.
package vscope

import (
	"sync"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

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
}

var activeVerifyContextCache sync.Map

// beginVerifyContextCache enables read-only memoization for ctx across a Pipeline.Execute pass.
func BeginVerifyContextCache(ctx *VerifyContext) {
	if ctx == nil {
		return
	}
	activeVerifyContextCache.Store(ctx, &verifyContextMemo{
		slicesByKey: make(map[string][]string),
		renameMaps:  make(map[string]map[string]string),
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
