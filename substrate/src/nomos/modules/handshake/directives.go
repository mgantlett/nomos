// Package handshake implements JIT session orientation telemetry, CapBAC tokens,
// NCTE-100 prose validation, and embedded JIT phase directives.
package handshake

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"gopkg.in/yaml.v3"
)

// EmbeddedDirectives embeds the canonical JIT directives within the compiled substrate binary.
//
//go:embed directives/*
var EmbeddedDirectives embed.FS

// DirectiveInfo summarizes an embedded or local directive's metadata.
type DirectiveInfo struct {
	Name     string           `json:"name"`
	Title    string           `json:"title"`
	Summary  string           `json:"summary"`
	Category string           `json:"category,omitempty"`
	Priority string           `json:"priority,omitempty"`
	Phases   []DirectivePhase `json:"phases,omitempty"`
	Source   string           `json:"source"` // "embedded" or "local"
}

// LoadPhaseDirectives loads matching directives for the given workspace phase.
// @intent INT-840 #DirectiveFrontmatter
func LoadPhaseDirectives(repoRoot string, phase types.Phase) []string {
	return loadPhaseDirectives(repoRoot, phase)
}

func isMarkdownDirective(name string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(name)), ".md")
}

func loadPhaseDirectives(repoRoot string, phase types.Phase) []string {
	phasePrefix := strings.ToLower(string(phase))

	if local := readLocalPhaseDirectives(repoRoot, phasePrefix); len(local) > 0 {
		sort.Strings(local)
		return local
	}

	var directives []string
	if entries, err := EmbeddedDirectives.ReadDir("directives"); err == nil {
		directives = filterPhaseMarkdownEntries(entries, phasePrefix)
	}
	if len(directives) == 0 {
		directives = matchDirectivesByFrontmatterPhase(repoRoot, phase)
	}
	if len(directives) == 0 {
		directives = append(directives, fmt.Sprintf("phase_%s_discipline", phasePrefix))
	}
	sort.Strings(directives)
	return directives
}

func readLocalPhaseDirectives(repoRoot, phasePrefix string) []string {
	if repoRoot == "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(repoRoot, "intent", "policy", "directives"))
	if err != nil {
		return nil
	}
	return filterPhaseMarkdownEntries(entries, phasePrefix)
}

func filterPhaseMarkdownEntries(entries []os.DirEntry, phasePrefix string) []string {
	var out []string
	for _, e := range entries {
		if !e.IsDir() && isMarkdownDirective(e.Name()) && strings.HasPrefix(strings.ToLower(e.Name()), phasePrefix) {
			out = append(out, e.Name())
		}
	}
	return out
}

func matchDirectivesByFrontmatterPhase(repoRoot string, phase types.Phase) []string {
	infos, err := ListDirectives(repoRoot)
	if err != nil {
		return nil
	}
	target := DirectivePhase(strings.ToUpper(string(phase)))
	var matched []string
	for _, info := range infos {
		for _, p := range info.Phases {
			if DirectivePhase(strings.ToUpper(string(p))) == target {
				matched = append(matched, info.Name)
				break
			}
		}
	}
	return matched
}

// GetEmbeddedDirective retrieves the byte content of an embedded directive by name.
// Matches exact filenames (e.g. "00_index_and_taxonomy.md") or prefixes/basenames without extension.
func GetEmbeddedDirective(name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "directives/")
	path := "directives/" + name
	if data, err := EmbeddedDirectives.ReadFile(path); err == nil {
		return data, nil
	}
	if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".yaml") {
		if data, err := EmbeddedDirectives.ReadFile(path + ".md"); err == nil {
			return data, nil
		}
	}

	entries, err := EmbeddedDirectives.ReadDir("directives")
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeBoundary, "failed reading embedded directives: %v", err)
	}

	target := strings.ToLower(name)
	for _, e := range entries {
		if e.IsDir() || !isMarkdownDirective(e.Name()) {
			continue
		}
		base := strings.ToLower(e.Name())
		baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))
		if base == target || baseNoExt == target || strings.HasPrefix(base, target) {
			return EmbeddedDirectives.ReadFile("directives/" + e.Name())
		}
	}

	return nil, types.NewNomosError(types.ErrCodeBoundary, "directive %q not found in embedded substrate directives", name)
}

// ListEmbeddedDirectives returns a sorted slice of all embedded Markdown directive names.
func ListEmbeddedDirectives() ([]string, error) {
	entries, err := EmbeddedDirectives.ReadDir("directives")
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeBoundary, "failed reading embedded directives: %v", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && isMarkdownDirective(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// GetDirective retrieves the content and source ("local" or "embedded") of a directive,
// checking local repoRoot overrides first and falling back to embedded directives.
func GetDirective(repoRoot, name string) ([]byte, string, error) {
	name = strings.TrimPrefix(name, "directives/")
	if repoRoot != "" {
		localDir := filepath.Join(repoRoot, "intent", "policy", "directives")
		localPath := filepath.Join(localDir, name)
		if isMarkdownDirective(name) {
			if data, err := os.ReadFile(localPath); err == nil {
				return data, "local", nil
			}
		} else {
			if data, err := os.ReadFile(localPath + ".md"); err == nil {
				return data, "local", nil
			}
		}

		if entries, err := os.ReadDir(localDir); err == nil {
			target := strings.ToLower(name)
			for _, e := range entries {
				if e.IsDir() || !isMarkdownDirective(e.Name()) {
					continue
				}
				base := strings.ToLower(e.Name())
				baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))
				if base == target || baseNoExt == target || strings.HasPrefix(base, target) {
					if data, rErr := os.ReadFile(filepath.Join(localDir, e.Name())); rErr == nil {
						return data, "local", nil
					}
				}
			}
		}
	}

	data, err := GetEmbeddedDirective(name)
	if err != nil {
		return nil, "", err
	}
	return data, "embedded", nil
}

// ListDirectives inspects all available Markdown directives (embedded and local), parsing their frontmatter.
func ListDirectives(repoRoot string) ([]DirectiveInfo, error) {
	embeddedNames, err := ListEmbeddedDirectives()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var list []DirectiveInfo

	if repoRoot != "" {
		localDir := filepath.Join(repoRoot, "intent", "policy", "directives")
		if entries, rErr := os.ReadDir(localDir); rErr == nil {
			for _, e := range entries {
				if e.IsDir() || !isMarkdownDirective(e.Name()) {
					continue
				}
				content, err := os.ReadFile(filepath.Join(localDir, e.Name()))
				if err != nil {
					continue
				}
				info := parseDirectiveFrontmatter(e.Name(), content, "local")
				list = append(list, info)
				seen[e.Name()] = true
			}
		}
	}

	for _, name := range embeddedNames {
		if seen[name] {
			continue
		}
		content, err := GetEmbeddedDirective(name)
		if err != nil {
			continue
		}
		info := parseDirectiveFrontmatter(name, content, "embedded")
		list = append(list, info)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	return list, nil
}

func parseDirectiveFrontmatter(name string, content []byte, source string) DirectiveInfo {
	info := DirectiveInfo{
		Name:   name,
		Source: source,
	}

	parts := strings.SplitN(string(content), "---", 3)
	if len(parts) >= 3 {
		var fm DirectiveFrontmatter
		if err := yaml.Unmarshal([]byte(parts[1]), &fm); err == nil {
			info.Title = fm.Title
			info.Summary = fm.Summary
			if info.Summary == "" {
				info.Summary = fm.Description
			}
			info.Category = fm.Category
			info.Priority = fm.Priority
			info.Phases = fm.Phases
		}
	}

	if info.Title == "" {
		info.Title = strings.TrimSuffix(name, filepath.Ext(name))
	}

	return info
}
