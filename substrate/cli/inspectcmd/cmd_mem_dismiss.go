// Package inspectcmd implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package inspectcmd

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"io"
	"strconv"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/spf13/cobra"
)

func newMemDismissCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "dismiss <id> [id...]",
		Short:   "Dismiss graduated or obsolete Nomos Mneme memory records",
		Long:    tui.HelpMemDismissLong,
		Example: tui.HelpMemDismissExample,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemDismiss(cmd.OutOrStdout(), args, jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, tui.FlagNameJSON, false, "Emit dismissed memory records as structured JSON")
	return cmd
}

func runMemDismiss(out io.Writer, rawArgs []string, jsonOutput bool) error {
	ids, err := parseMemoryDismissIDs(rawArgs)
	if err != nil {
		return err
	}

	store, err := mem.NewStore(config.ResolveMnemeDBPath())
	if err != nil {
		return err
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()

	dismissed := make([]*mem.Memory, 0, len(ids))
	for _, id := range ids {
		rec, dErr := store.Dismiss(id)
		if dErr != nil {
			return dErr
		}
		dismissed = append(dismissed, rec)
	}

	if jsonOutput {
		data, mErr := json.MarshalIndent(dismissed, "", "  ")
		if mErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed marshaling dismissed memories JSON: %v", mErr)
		}
		fmt.Fprintln(out, string(data))
		return nil
	}

	for _, rec := range dismissed {
		fmt.Fprintf(out, "🗃️  %s #%d [%s/%s] (%s)\n",
			tui.UiSuccessStyle.Render("Dismissed memory"),
			rec.ID,
			tui.UiSubstrateStyle.Render(rec.RepoID),
			tui.UiValueStyle.Render(rec.Topic),
			tui.UiMutedStyle.Render(string(rec.Status)))
	}
	return nil
}

func parseMemoryDismissIDs(rawArgs []string) ([]int, error) {
	seen := make(map[int]bool, len(rawArgs))
	var ids []int
	for _, raw := range rawArgs {
		for _, tok := range strings.Split(raw, ",") {
			clean := strings.TrimPrefix(strings.TrimSpace(tok), "#")
			if clean == "" {
				continue
			}
			id, err := strconv.Atoi(clean)
			if err != nil || id <= 0 {
				return nil, types.NewNomosError(types.ErrCodeValidationFailed, "invalid memory ID %q: must be a positive integer", tok).
					WithRemedy("run 'nomos mem list' to inspect valid numeric memory IDs")
			}
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "at least one numeric memory ID is required")
	}
	return ids, nil
}

func resolveMemListStatusFilter(raw string) (mem.MemoryStatus, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(mem.StatusActive):
		return mem.StatusActive, nil
	case string(mem.StatusSuperseded):
		return mem.StatusSuperseded, nil
	case string(mem.StatusTombstoned):
		return mem.StatusTombstoned, nil
	case "all":
		return "", nil
	default:
		return "", types.NewNomosError(types.ErrCodeValidationFailed,
			"invalid memory status %q: expected active, superseded, tombstoned, or all", raw)
	}
}
