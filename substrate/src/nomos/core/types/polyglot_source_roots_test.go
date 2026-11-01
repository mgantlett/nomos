package types

import (
	"context"
	"strings"
	"testing"
)

// TestVEC_ROOT_SourceRootVectors verifies the 8 table-driven test vectors from polyglot_source_roots_spec.md.
func TestVEC_ROOT_SourceRootVectors(t *testing.T) {
	ctx := context.Background()

	t.Run("VEC-ROOT-01 Longest Prefix Match Go", func(t *testing.T) {
		manifest := &PolyglotWorkspaceManifest{
			SourceRoots: []SourceRootConfig{
				{Path: "substrate/src/nomos", Language: LanguageGo},
			},
		}
		resolver := NewSourceRootResolver(manifest)
		res, err := resolver.Resolve(ctx, "substrate/src/nomos/main.go")
		if err != nil {
			t.Fatalf("expected VEC-ROOT-01 to pass, got err: %v", err)
		}
		if res.MatchedRoot.Path != "substrate/src/nomos" || res.Language != LanguageGo {
			t.Errorf("unexpected match: %+v", res)
		}
		if res.IsExcluded {
			t.Errorf("expected not excluded")
		}
	})

	t.Run("VEC-ROOT-02 Svelte Membrane Root Binding", func(t *testing.T) {
		manifest := &PolyglotWorkspaceManifest{
			SourceRoots: []SourceRootConfig{
				{Path: "substrate/membrane/cockpit", Language: LanguageSvelte},
			},
		}
		resolver := NewSourceRootResolver(manifest)
		res, err := resolver.Resolve(ctx, "substrate/membrane/cockpit/src/App.svelte")
		if err != nil {
			t.Fatalf("expected VEC-ROOT-02 to pass, got err: %v", err)
		}
		if res.MatchedRoot.Path != "substrate/membrane/cockpit" || res.Language != LanguageSvelte {
			t.Errorf("unexpected match: %+v", res)
		}
	})

	t.Run("VEC-ROOT-03 Exclude Pattern Match NodeModules", func(t *testing.T) {
		manifest := &PolyglotWorkspaceManifest{
			SourceRoots: []SourceRootConfig{
				{
					Path:            "substrate/membrane/cockpit",
					Language:        LanguageSvelte,
					ExcludePatterns: []string{"**/node_modules/**"},
				},
			},
		}
		resolver := NewSourceRootResolver(manifest)
		res, err := resolver.Resolve(ctx, "substrate/membrane/cockpit/node_modules/pkg/index.js")
		if err != nil {
			t.Fatalf("expected VEC-ROOT-03 resolve without error, got: %v", err)
		}
		if !res.IsExcluded {
			t.Errorf("expected VEC-ROOT-03 to be excluded (SKIP)")
		}
	})

	t.Run("VEC-ROOT-04 Law 5 Leading Slash Absolute Path Rejected", func(t *testing.T) {
		manifest := &PolyglotWorkspaceManifest{
			SourceRoots: []SourceRootConfig{
				{Path: "substrate", Language: LanguageGo},
			},
		}
		resolver := NewSourceRootResolver(manifest)
		_, err := resolver.Resolve(ctx, "/etc/shadow")
		if err == nil {
			t.Fatalf("expected VEC-ROOT-04 absolute path to fail closed")
		}
		if !strings.Contains(err.Error(), "Law 5") {
			t.Errorf("expected Law 5 error, got: %v", err)
		}
	})

	t.Run("VEC-ROOT-05 Law 5 Parent Traversal Rejected", func(t *testing.T) {
		manifest := &PolyglotWorkspaceManifest{
			SourceRoots: []SourceRootConfig{
				{Path: "substrate", Language: LanguageGo},
			},
		}
		resolver := NewSourceRootResolver(manifest)
		_, err := resolver.Resolve(ctx, "../../outside.go")
		if err == nil {
			t.Fatalf("expected VEC-ROOT-05 traversal to fail closed")
		}
		if !strings.Contains(err.Error(), "Law 5") {
			t.Errorf("expected Law 5 error, got: %v", err)
		}
	})

	t.Run("VEC-ROOT-06 Law 7 Unmapped Source Root Fails Closed Strict", func(t *testing.T) {
		manifest := &PolyglotWorkspaceManifest{
			SourceRoots: []SourceRootConfig{
				{Path: "substrate", Language: LanguageGo},
			},
			StrictPolyglot: true,
		}
		resolver := NewSourceRootResolver(manifest)
		_, err := resolver.Resolve(ctx, "scripts/tool.py")
		if err == nil {
			t.Fatalf("expected VEC-ROOT-06 to fail closed under strict mode")
		}
		if !strings.Contains(err.Error(), "Law 7") {
			t.Errorf("expected Law 7 error, got: %v", err)
		}
	})

	t.Run("VEC-ROOT-07 Longest Prefix Match Wins Over Parent", func(t *testing.T) {
		manifest := &PolyglotWorkspaceManifest{
			SourceRoots: []SourceRootConfig{
				{Path: "substrate", Language: LanguageGo},
				{Path: "substrate/membrane", Language: LanguageTypeScript},
			},
		}
		resolver := NewSourceRootResolver(manifest)
		res, err := resolver.Resolve(ctx, "substrate/membrane/nested/file.ts")
		if err != nil {
			t.Fatalf("expected VEC-ROOT-07 to pass, got: %v", err)
		}
		if res.MatchedRoot.Path != "substrate/membrane" || res.Language != LanguageTypeScript {
			t.Errorf("expected substrate/membrane with typescript, got: %+v", res)
		}
	})

	t.Run("VEC-ROOT-08 Intent Plane Markdown AST Parsing", func(t *testing.T) {
		manifest := &PolyglotWorkspaceManifest{
			SourceRoots: []SourceRootConfig{
				{Path: "intent", Language: LanguageMarkdown},
			},
		}
		resolver := NewSourceRootResolver(manifest)
		res, err := resolver.Resolve(ctx, "intent/architecture/spec.md")
		if err != nil {
			t.Fatalf("expected VEC-ROOT-08 to pass, got: %v", err)
		}
		if res.MatchedRoot.Path != "intent" || res.Language != LanguageMarkdown {
			t.Errorf("expected intent with markdown, got: %+v", res)
		}
	})
}
