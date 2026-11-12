// Package tui implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package tui

import (
	"io"
	"os"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/spf13/cobra"
)

type TaskCardFlags struct {
	File      string
	Desc      string
	Project   string
	Scope     string
	IntScope  string
	SubScope  string
	AC        []string
	Verify    string
	DependsOn []string
	Burden    int
}

// BindTaskCardFlags executes deterministic tui verification logic (@intent task_lifecycle_cli_spec.md).
func BindTaskCardFlags(cmd *cobra.Command, f *TaskCardFlags, includePairScopes bool) {
	cmd.Flags().StringVarP(&f.File, "file", "F", "", "Path to 3-section Markdown card file (use '-' for stdin)")
	cmd.Flags().StringVarP(&f.Desc, "desc", "d", "", "3-section Markdown task description")
	cmd.Flags().StringVar(&f.Project, "project", "", "Repository project identifier (defaults to active repository)")
	cmd.Flags().StringVar(&f.Scope, "scope", "", "Target files/specs and problem context ('## Scope & Context')")
	cmd.Flags().StringArrayVar(&f.AC, "ac", nil, "Deliverable checklist item for '## Acceptance Criteria' (repeatable)")
	cmd.Flags().StringVar(&f.Verify, "verify", "", "Test command or proof for '## Verification'")
	cmd.Flags().StringSliceVar(&f.DependsOn, "depends-on", nil, "Blocker task key(s) in causal DAG (comma-separated or repeatable)")
	cmd.Flags().IntVarP(&f.Burden, "burden", "b", 0, "Explicit cognitive burden 1-10 (0 = auto-derive from card density)")
	if includePairScopes {
		cmd.Flags().StringVar(&f.IntScope, "int-scope", "", "Intent-specific override for '## Scope & Context'")
		cmd.Flags().StringVar(&f.SubScope, "sub-scope", "", "Substrate-specific override for '## Scope & Context'")
	}
}

// ResolveValidatedDependencies executes deterministic tui verification logic (@intent task_lifecycle_cli_spec.md).
func ResolveValidatedDependencies(store task.TaskStore, raw []string, selfKeys ...string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	selfSet := make(map[string]bool, len(selfKeys))
	for _, sk := range selfKeys {
		selfSet[strings.ToUpper(strings.TrimSpace(sk))] = true
	}
	seen := make(map[string]bool, len(raw))
	var out []string
	for _, item := range raw {
		for _, part := range strings.Split(item, ",") {
			dep := strings.ToUpper(strings.TrimSpace(part))
			if dep == "" || seen[dep] {
				continue
			}
			if err := task.ValidateTaskKey(dep); err != nil {
				return nil, err
			}
			if selfSet[dep] {
				return nil, types.NewNomosError(types.ErrCodeValidationFailed, "task cannot depend on itself (%s)", dep)
			}
			if _, err := store.GetTask(dep); err != nil {
				return nil, types.NewNomosError(types.ErrCodeValidationFailed, "dependency task %s does not exist", dep)
			}
			seen[dep] = true
			out = append(out, dep)
		}
	}
	return out, nil
}

// MaterializeCardStdin executes deterministic tui verification logic (@intent task_lifecycle_cli_spec.md).
func MaterializeCardStdin(stdin io.Reader, f *TaskCardFlags) error {
	if strings.TrimSpace(f.File) != "-" {
		return nil
	}
	if stdin == nil {
		stdin = os.Stdin
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "failed reading task card from stdin: %v", err)
	}
	f.Desc = string(data)
	f.File = ""
	return nil
}

// ResolveCardBodyAndBurden executes deterministic tui verification logic (@intent task_lifecycle_cli_spec.md).
func ResolveCardBodyAndBurden(stdin io.Reader, subcommand, title, plane string, f TaskCardFlags, scopeOverride string) (string, int, error) {
	effectiveScope := strings.TrimSpace(scopeOverride)
	if effectiveScope == "" {
		effectiveScope = strings.TrimSpace(f.Scope)
	}
	hasStructuredFlags := effectiveScope != "" || len(f.AC) > 0 || strings.TrimSpace(f.Verify) != ""

	var body string
	var err error
	if hasStructuredFlags {
		body, err = task.RenderTaskCardBody(task.TaskCardInput{
			Scope:              effectiveScope,
			AcceptanceCriteria: f.AC,
			Verification:       f.Verify,
		})
	} else {
		body, err = readRawCardBody(stdin, f.File, f.Desc)
		if err == nil {
			err = task.ValidateTaskCardBody(body)
		}
	}
	if err != nil {
		return "", 0, AttachCardRemedy(err, subcommand, title, plane)
	}

	burden, bErr := task.DeriveTaskBurden(f.Burden, body)
	if bErr != nil {
		return "", 0, bErr
	}
	return body, burden, nil
}

func readRawCardBody(stdin io.Reader, fileFlag, descFlag string) (string, error) {
	trimmedFile := strings.TrimSpace(fileFlag)
	if trimmedFile == "-" {
		if stdin == nil {
			stdin = os.Stdin
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", types.NewNomosError(types.ErrCodeValidationFailed, "failed reading task card from stdin: %v", err)
		}
		return string(data), nil
	}
	if trimmedFile != "" {
		data, err := os.ReadFile(trimmedFile)
		if err != nil {
			return "", types.NewNomosError(types.ErrCodeValidationFailed, "failed reading task card file %q: %v", trimmedFile, err)
		}
		return string(data), nil
	}
	return descFlag, nil
}

// AttachCardRemedy executes deterministic tui verification logic (@intent task_lifecycle_cli_spec.md).
func AttachCardRemedy(err error, subcommand, title, plane string) error {
	remedy := task.TaskCardRemedy(subcommand, title, plane)
	if nErr, ok := err.(*types.NomosError); ok {
		return nErr.WithRemedy(remedy)
	}
	return types.NewNomosError(types.ErrCodeValidationFailed, "%v", err).WithRemedy(remedy)
}

// NormalizePlaneInput normalizes shorthand INT/SUB plane flags against optional explicit task keys (@intent task_lifecycle_cli_spec.md).
func NormalizePlaneInput(plane, customKey string) string {
	switch strings.ToLower(strings.TrimSpace(plane)) {
	case "sub", "substrate":
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(customKey)), "INT-") {
			return "intent"
		}
		return "substrate"
	case "int", "intent":
		return "intent"
	default:
		return strings.ToLower(strings.TrimSpace(plane))
	}
}
