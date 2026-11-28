// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mgantlett/nomos/substrate/cli/tui"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/sarif"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/xray"
	"github.com/spf13/cobra"
)

type xrayCLIOptions struct {
	diffBaseRef    string
	topHotspots    int
	jsonOutput     bool
	markdownOutput bool
	sarifOutput    bool
	badgeOutput    bool
	badgeSVGOutput bool
}

// NewXrayCmd constructs the zero-init read-only 'nomos xray' command (@intent task_lifecycle_cli_spec.md).
func NewXrayCmd() *cobra.Command {
	var opts xrayCLIOptions

	cmd := &cobra.Command{
		Use:     "xray [dir]",
		Short:   "Run zero-init read-only brownfield AI-hangover & structural debt diagnostics",
		Long:    tui.HelpXrayLong,
		Example: tui.HelpXrayExample,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateXrayOutputFlags(opts); err != nil {
				return err
			}
			targetDir, err := resolveXrayTargetDir(args)
			if err != nil {
				return err
			}
			return executeXray(cmd.OutOrStdout(), targetDir, opts)
		},
	}

	cmd.Flags().StringVar(&opts.diffBaseRef, "diff", "", "Restrict X-Ray scan to files added or modified since the given Git ref plus untracked files")
	cmd.Flags().BoolVar(&opts.jsonOutput, tui.FlagNameJSON, false, "Emit structured XrayReport as JSON")
	cmd.Flags().BoolVar(&opts.markdownOutput, "markdown", false, "Emit shareable GitHub Markdown diagnostic card")
	cmd.Flags().BoolVar(&opts.sarifOutput, "sarif", false, "Emit OASIS SARIF 2.1.0 JSON report for GitHub Code Scanning")
	cmd.Flags().BoolVar(&opts.badgeOutput, "badge", false, "Emit a shareable Shields.io Markdown score badge snippet")
	cmd.Flags().BoolVar(&opts.badgeSVGOutput, "badge-svg", false, "Emit a standalone deterministic SVG score badge")
	cmd.Flags().IntVar(&opts.topHotspots, "top", xray.DefaultTopHotspots, "Maximum number of ranked structural hotspots to display")
	return cmd
}

func validateXrayOutputFlags(opts xrayCLIOptions) error {
	count := 0
	for _, set := range []bool{opts.jsonOutput, opts.markdownOutput, opts.sarifOutput, opts.badgeOutput, opts.badgeSVGOutput} {
		if set {
			count++
		}
	}
	if count > 1 {
		return types.NewNomosError(types.ErrCodeValidationFailed, "cannot combine --json, --markdown, --sarif, --badge, and --badge-svg on nomos xray").
			WithRemedy("specify at most one output flag (--json, --markdown, --sarif, --badge, or --badge-svg)")
	}
	return nil
}

func resolveXrayTargetDir(args []string) (string, error) {
	target := "."
	if len(args) > 0 && args[0] != "" {
		target = args[0]
	} else if cwd, err := os.Getwd(); err == nil {
		target = cwd
	}
	absDir, err := filepath.Abs(target)
	if err != nil {
		return "", types.NewNomosError(types.ErrCodeBoundary, "failed resolving target directory %s: %v", target, err)
	}
	return absDir, nil
}

func executeXray(out io.Writer, targetDir string, opts xrayCLIOptions) error {
	rep, err := xray.ScanRepository(targetDir, xray.XrayScanOptions{
		TopHotspots: opts.topHotspots,
		DiffBaseRef: opts.diffBaseRef,
	})
	if err != nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "xray scan failed: %v", err).
			WithRemedy("pass an existing repository or valid Git ref to 'nomos xray [dir] [--diff <ref>]'")
	}
	if opts.jsonOutput {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	if opts.sarifOutput {
		raw, sErr := sarif.MarshalIndent(sarif.FromXrayReport(rep, RootCmdVersion))
		if sErr != nil {
			return sErr
		}
		_, wErr := fmt.Fprintln(out, string(raw))
		return wErr
	}
	if opts.badgeOutput {
		_, wErr := fmt.Fprintln(out, xray.FormatBadgeMarkdown(rep))
		return wErr
	}
	if opts.badgeSVGOutput {
		_, wErr := fmt.Fprintln(out, xray.FormatBadgeSVG(rep))
		return wErr
	}
	if opts.markdownOutput {
		_, wErr := fmt.Fprint(out, xray.FormatMarkdown(rep))
		return wErr
	}
	_, wErr := fmt.Fprint(out, tui.RenderXrayTerminal(rep))
	return wErr
}
