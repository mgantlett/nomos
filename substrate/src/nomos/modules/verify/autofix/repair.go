// Package autofix implements deterministic repair recipe extraction and safe mechanical auto-fixes.
// @intent task_lifecycle_cli_spec.md
package autofix

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

const nomosSchemaFileName = "nomos.schema.json"

// VerifyFixRecipe describes a single deterministic remediation step extracted from a failed gate (#VerifyFixRecipe).
type VerifyFixRecipe struct {
	GateName    string `json:"gate_name"`
	ProbeID     string `json:"probe_id,omitempty"`
	Law         int    `json:"law,omitempty"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	ActionKind  string `json:"action_kind"`
	SafeAutoFix bool   `json:"safe_auto_fix"`
	Command     string `json:"command,omitempty"`
	PatchHint   string `json:"patch_hint"`
}

// VerifyFixPlan aggregates deterministic remediation recipes and applied mechanical fixes (#VerifyFixPlan).
type VerifyFixPlan struct {
	Passed       bool              `json:"passed"`
	GatesPassed  int               `json:"gates_passed"`
	GatesTotal   int               `json:"gates_total"`
	AppliedFixes []string          `json:"applied_fixes,omitempty"`
	Recipes      []VerifyFixRecipe `json:"recipes,omitempty"`
}

var fixFileLineRe = regexp.MustCompile(`([A-Za-z0-9_./-]+\.(?:go|ts|tsx|js|jsx|mjs|cjs|svelte|py|rs|rb|java|kt|cs|c|cc|cpp|h|hpp|json|yaml|yml|md)):(\d+)`)
var fixFileOnlyRe = regexp.MustCompile(`([A-Za-z0-9_./-]+\.(?:go|ts|tsx|js|jsx|mjs|cjs|svelte|py|rs|rb|java|kt|cs|c|cc|cpp|h|hpp|json|yaml|yml|md))`)

// BuildVerifyFixPlan distills failed gate outcomes into machine-actionable repair recipes.
func BuildVerifyFixPlan(passed bool, gatesPassed, gatesTotal int, results []*vscope.GateResult) *VerifyFixPlan {
	plan := &VerifyFixPlan{
		Passed:      passed,
		GatesPassed: gatesPassed,
		GatesTotal:  gatesTotal,
	}
	for _, res := range results {
		if res == nil || res.Passed || res.Skipped {
			continue
		}
		plan.Recipes = append(plan.Recipes, buildGateFixRecipe(res))
	}
	return plan
}

func buildGateFixRecipe(res *vscope.GateResult) VerifyFixRecipe {
	file, line := extractMessageLocation(res.Message)
	recipe := VerifyFixRecipe{
		GateName:   res.GateName,
		ProbeID:    res.ProbeID,
		Law:        res.Law,
		File:       file,
		Line:       line,
		ActionKind: "refactor",
		PatchHint:  strings.TrimSpace(res.Remedy),
	}
	if recipe.PatchHint == "" {
		recipe.PatchHint = strings.TrimSpace(res.Message)
	}
	classifyRecipeAction(&recipe, res)
	return recipe
}

func classifyRecipeAction(recipe *VerifyFixRecipe, res *vscope.GateResult) {
	msgLower := strings.ToLower(res.Message)
	switch {
	case res.ProbeID == "contract_parity" && strings.Contains(msgLower, nomosSchemaFileName):
		recipe.ActionKind = "auto_schema_sync"
		recipe.SafeAutoFix = true
		recipe.File = nomosSchemaFileName
		recipe.Command = "nomos verify --fix"
		if recipe.PatchHint == "" {
			recipe.PatchHint = "Regenerate nomos.schema.json from compiled config.EcosystemConfig AST"
		}
	case (res.ProbeID == "go_toolchain" || strings.Contains(strings.ToLower(res.GateName), "gofmt")) && strings.HasSuffix(recipe.File, ".go"):
		recipe.ActionKind = "auto_format"
		recipe.SafeAutoFix = true
		recipe.Command = "gofmt -w " + recipe.File
	case res.ProbeID != "":
		recipe.Command = "nomos capabilities " + res.ProbeID
	}
}

func extractMessageLocation(msg string) (string, int) {
	if m := fixFileLineRe.FindStringSubmatch(msg); len(m) == 3 {
		lineNum, err := strconv.Atoi(m[2])
		if err != nil {
			lineNum = 0
		}
		return filepath.ToSlash(m[1]), lineNum
	}
	if m := fixFileOnlyRe.FindStringSubmatch(msg); len(m) == 2 {
		return filepath.ToSlash(m[1]), 0
	}
	return "", 0
}

// ApplySafeAutoFixes executes only deterministic, lossless mechanical repairs confined to worktreeDir.
func ApplySafeAutoFixes(worktreeDir string, isBaseline bool, plan *VerifyFixPlan) ([]string, error) {
	if isBaseline {
		return nil, types.NewNomosError(types.ErrCodeBoundary, "cannot run --fix on protected trunk/baseline").
			WithRemedy("switch to an active feature branch or isolated task worktree before running 'nomos verify --fix'")
	}
	if plan == nil {
		return nil, nil
	}
	absRoot, err := filepath.Abs(worktreeDir)
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeBoundary, "failed resolving worktree directory: %v", err)
	}
	var applied []string
	schemaFixed := false
	for _, r := range plan.Recipes {
		if !r.SafeAutoFix {
			continue
		}
		if r.ActionKind == "auto_schema_sync" && !schemaFixed {
			if msg, ok, sErr := syncCanonicalConfigSchema(absRoot); sErr != nil {
				return applied, sErr
			} else if ok {
				applied = append(applied, msg)
				schemaFixed = true
			}
		}
	}
	formatted, fmtErr := formatModifiedGoFiles(absRoot)
	if fmtErr != nil {
		return applied, fmtErr
	}
	applied = append(applied, formatted...)
	return applied, nil
}

func syncCanonicalConfigSchema(absRoot string) (string, bool, error) {
	schemaPath := filepath.Join(absRoot, nomosSchemaFileName)
	existing, statErr := os.ReadFile(schemaPath)
	if statErr != nil {
		return "", false, nil
	}
	canonical, genErr := config.GenerateConfigJSONSchema()
	if genErr != nil {
		return "", false, types.NewNomosError(types.ErrCodeCore, "failed generating canonical config schema: %v", genErr)
	}
	if bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(canonical)) {
		return "", false, nil
	}
	if wErr := os.WriteFile(schemaPath, canonical, 0644); wErr != nil {
		return "", false, types.NewNomosError(types.ErrCodeSubstrate, "failed writing %s: %v", nomosSchemaFileName, wErr)
	}
	return "synced " + nomosSchemaFileName + " from Go SSoT", true, nil
}

func formatModifiedGoFiles(absRoot string) ([]string, error) {
	paths := collectModifiedGoPaths(absRoot)
	var applied []string
	for _, rel := range paths {
		absPath := filepath.Join(absRoot, filepath.FromSlash(rel))
		cleanTarget := filepath.Clean(absPath)
		if !strings.HasPrefix(cleanTarget, filepath.Clean(absRoot)+string(filepath.Separator)) {
			continue
		}
		raw, rErr := os.ReadFile(cleanTarget)
		if rErr != nil {
			continue
		}
		formatted, fErr := format.Source(raw)
		if fErr != nil || bytes.Equal(raw, formatted) {
			continue
		}
		if wErr := os.WriteFile(cleanTarget, formatted, 0644); wErr != nil {
			return applied, types.NewNomosError(types.ErrCodeSubstrate, "failed writing gofmt output for %s: %v", rel, wErr)
		}
		applied = append(applied, "gofmt "+rel)
	}
	return applied, nil
}

func collectModifiedGoPaths(absRoot string) []string {
	seen := make(map[string]bool)
	queries := [][]string{
		{"diff", "--name-only", "--diff-filter=ACMR", "HEAD", "--", "."},
		{"ls-files", "--others", "--exclude-standard", "--", "."},
	}
	for _, args := range queries {
		out, err := workspace.SafeGitCommand(absRoot, args...).Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(out), "\n") {
			rel := filepath.ToSlash(strings.TrimSpace(line))
			if rel != "" && strings.HasSuffix(rel, ".go") && !strings.HasPrefix(rel, "..") {
				seen[rel] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	return out
}

// FormatFixPlanTerminal renders a concise numbered repair checklist for CLI and agent consumption.
func FormatFixPlanTerminal(plan *VerifyFixPlan) string {
	if plan == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("🔧 Nomos Deterministic Repair Plan\n")
	sb.WriteString(strings.Repeat("─", 76) + "\n")
	for _, a := range plan.AppliedFixes {
		sb.WriteString(fmt.Sprintf("  ✅ Applied Safe Auto-Fix: %s\n", a))
	}
	if len(plan.Recipes) == 0 {
		sb.WriteString(fmt.Sprintf("  ✅ All %d/%d verification gates satisfied (0 repair recipes)\n", plan.GatesPassed, plan.GatesTotal))
		return sb.String()
	}
	for i, r := range plan.Recipes {
		loc := formatRecipeLocation(r.File, r.Line)
		tag := r.ActionKind
		if r.SafeAutoFix {
			tag += ", auto-fixable"
		}
		sb.WriteString(fmt.Sprintf("  %d. [%s] (%s)%s\n", i+1, r.GateName, tag, loc))
		sb.WriteString(fmt.Sprintf("     Hint   : %s\n", r.PatchHint))
		if r.Command != "" {
			sb.WriteString(fmt.Sprintf("     Command: %s\n", r.Command))
		}
	}
	return sb.String()
}

func formatRecipeLocation(file string, line int) string {
	if file == "" {
		return ""
	}
	if line > 0 {
		return fmt.Sprintf(" %s:%d", file, line)
	}
	return " " + file
}
