// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"io"
	"os"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/spf13/cobra"
)

type taskCardFlags struct {
	file      string
	desc      string
	project   string
	scope     string
	intScope  string
	subScope  string
	ac        []string
	verify    string
	dependsOn []string
	burden    int
}

func bindTaskCardFlags(cmd *cobra.Command, f *taskCardFlags, includePairScopes bool) {
	cmd.Flags().StringVarP(&f.file, "file", "F", "", "Path to 3-section Markdown card file (use '-' for stdin)")
	cmd.Flags().StringVarP(&f.desc, "desc", "d", "", "3-section Markdown task description")
	cmd.Flags().StringVar(&f.project, "project", "", "Repository project identifier (defaults to active repository)")
	cmd.Flags().StringVar(&f.scope, "scope", "", "Target files/specs and problem context ('## Scope & Context')")
	cmd.Flags().StringArrayVar(&f.ac, "ac", nil, "Deliverable checklist item for '## Acceptance Criteria' (repeatable)")
	cmd.Flags().StringVar(&f.verify, "verify", "", "Test command or proof for '## Verification'")
	cmd.Flags().StringSliceVar(&f.dependsOn, "depends-on", nil, "Blocker task key(s) in causal DAG (comma-separated or repeatable)")
	cmd.Flags().IntVarP(&f.burden, "burden", "b", 0, "Explicit cognitive burden 1-10 (0 = auto-derive from card density)")
	if includePairScopes {
		cmd.Flags().StringVar(&f.intScope, "int-scope", "", "Intent-specific override for '## Scope & Context'")
		cmd.Flags().StringVar(&f.subScope, "sub-scope", "", "Substrate-specific override for '## Scope & Context'")
	}
}

func resolveValidatedDependencies(store task.TaskStore, raw []string, selfKeys ...string) ([]string, error) {
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

func materializeCardStdin(stdin io.Reader, f *taskCardFlags) error {
	if strings.TrimSpace(f.file) != "-" {
		return nil
	}
	if stdin == nil {
		stdin = os.Stdin
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "failed reading task card from stdin: %v", err)
	}
	f.desc = string(data)
	f.file = ""
	return nil
}

func resolveCardBodyAndBurden(stdin io.Reader, subcommand, title, plane string, f taskCardFlags, scopeOverride string) (string, int, error) {
	effectiveScope := strings.TrimSpace(scopeOverride)
	if effectiveScope == "" {
		effectiveScope = strings.TrimSpace(f.scope)
	}
	hasStructuredFlags := effectiveScope != "" || len(f.ac) > 0 || strings.TrimSpace(f.verify) != ""

	var body string
	var err error
	if hasStructuredFlags {
		body, err = task.RenderTaskCardBody(task.TaskCardInput{
			Scope:              effectiveScope,
			AcceptanceCriteria: f.ac,
			Verification:       f.verify,
		})
	} else {
		body, err = readRawCardBody(stdin, f.file, f.desc)
		if err == nil {
			err = task.ValidateTaskCardBody(body)
		}
	}
	if err != nil {
		return "", 0, attachCardRemedy(err, subcommand, title, plane)
	}

	burden, bErr := task.DeriveTaskBurden(f.burden, body)
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

func attachCardRemedy(err error, subcommand, title, plane string) error {
	remedy := task.TaskCardRemedy(subcommand, title, plane)
	if nErr, ok := err.(*types.NomosError); ok {
		return nErr.WithRemedy(remedy)
	}
	return types.NewNomosError(types.ErrCodeValidationFailed, "%v", err).WithRemedy(remedy)
}
