// Package inspectcmd implements deterministic Cobra command dispatch, software physics telemetry,
// session management, and terminal UI contracts for Nomos.
package inspectcmd

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/session"
	"github.com/spf13/cobra"
)

func newSessionExportCmd() *cobra.Command {
	var brainDir string
	var repoRoot string
	var limit int
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Export joined session trajectories, Sentinel PRM scores, receipts, and Mneme learnings",
		Long:    tui.HelpSessionExportLong,
		Example: tui.HelpSessionExportExample,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			records, err := session.ExportSessionDataset(brainDir, repoRoot, limit)
			if err != nil {
				return types.NewNomosError(types.ErrCodeBoundary, "failed exporting session dataset: %v", err)
			}
			out := cmd.OutOrStdout()
			if jsonOutput {
				data, mErr := json.MarshalIndent(records, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling dataset JSON: %v", mErr)
				}
				if _, wErr := fmt.Fprintln(out, string(data)); wErr != nil {
					return wErr
				}
				return nil
			}
			for _, rec := range records {
				line, mErr := json.Marshal(rec)
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling JSONL record: %v", mErr)
				}
				if _, wErr := fmt.Fprintln(out, string(line)); wErr != nil {
					return wErr
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&brainDir, "brain", "b", "", "Session storage or brain directory path override")
	cmd.Flags().StringVarP(&repoRoot, "root", "r", "", "Repository root directory path override")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of sessions to export")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Emit formatted JSON array instead of default JSONL lines")
	return cmd
}
