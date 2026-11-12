// Package inspectcmd implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package inspectcmd

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/spf13/cobra"
)

// NewMemCmd creates the authoritative 'nomos mem' command hierarchy.
func NewMemCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mem",
		Short: "Nomos Mneme receipt-gated episodic memory",
		Long: `Nomos Mneme receipt-gated episodic memory subsystem. Records reflections,
axioms, and architectural decisions with hybrid subword, lexical, and optional local embedding recall.`,
		Example: tui.HelpMemExample,
	}

	cmd.AddCommand(NewMemSaveCmd())
	cmd.AddCommand(newMemRecallCmd())
	cmd.AddCommand(newMemListCmd())
	cmd.AddCommand(newMemDismissCmd())
	return cmd
}

// NewMemSaveCmd executes deterministic inspectcmd verification logic (@intent task_lifecycle_cli_spec.md).
func NewMemSaveCmd() *cobra.Command {
	var kind, topic, project string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "save <content>",
		Short:   "Save knowledge reflection or lesson into Nomos Mneme",
		Long:    tui.HelpMemSaveLong,
		Example: tui.HelpMemSaveExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemSave(cmd.OutOrStdout(), args[0], kind, topic, project, jsonOutput)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", string(mem.KindReflection), "Advisory kind of memory (reflection, axiom, decision)")
	cmd.Flags().StringVar(&topic, "topic", "General", "Topic identifier")
	cmd.Flags().StringVarP(&project, tui.FlagNameProject, "p", "", "Repository identifier (defaults to active repo)")
	cmd.Flags().BoolVar(&jsonOutput, tui.FlagNameJSON, false, "Emit saved memory record as structured JSON")
	return cmd
}

func runMemSave(out io.Writer, content, kind, topic, project string, jsonOutput bool) error {
	if vErr := mem.ValidateAdvisoryWriteKind(mem.MemoryKind(kind)); vErr != nil {
		return vErr
	}
	store, err := mem.NewStore(config.ResolveMnemeDBPath())
	if err != nil {
		return err
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// safe to continue
		}
	}()

	repoID := project
	if repoID == "" {
		repoID = ResolveCurrentRepoID()
	}
	m := &mem.Memory{RepoID: repoID, Kind: mem.MemoryKind(kind), Topic: topic, Content: content}
	if err := store.Save(m); err != nil {
		return err
	}
	if jsonOutput {
		if data, mErr := json.MarshalIndent(m, "", "  "); mErr == nil {
			fmt.Fprintln(out, string(data))
			return nil
		}
	}

	fmt.Printf("🧠 %s #%d saved [%s/%s]: %s\n",
		tui.UiSuccessStyle.Render("Nomos Mneme memory"), m.ID,
		tui.UiSubstrateStyle.Render(m.RepoID), tui.UiValueStyle.Render(m.Topic),
		tui.UiIntentStyle.Render(string(m.Kind)))
	tui.RenderNextActions(os.Stdout, []types.ActionAffordance{
		{Command: fmt.Sprintf("nomos mem recall %q", m.Topic), Summary: "Verify indexed recall of saved memory"},
	})
	return nil
}

type recallArgs struct {
	topic, kind, project string
	limit                int
	jsonOutput           bool
	crossRepo            bool
}

func runMemRecall(query string, r recallArgs) error {
	store, err := mem.NewStore(config.ResolveMnemeDBPath())
	if err != nil {
		return err
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// safe to continue
		}
	}()

	repoID := r.project
	if !r.crossRepo && repoID == "" {
		repoID = ResolveCurrentRepoID()
	}

	opts := mem.SearchOptions{
		Limit:            r.limit,
		Kind:             mem.MemoryKind(r.kind),
		Topic:            r.topic,
		RepoID:           repoID,
		IncludeCrossRepo: r.crossRepo,
		ReadOnly:         true,
	}
	results, err := store.SearchWithOptions(query, opts)
	if err != nil {
		return err
	}

	if r.jsonOutput {
		data, mErr := json.MarshalIndent(results, "", "  ")
		if mErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed marshaling json: %v", mErr)
		}
		fmt.Println(string(data))
		return nil
	}

	renderRecallResults(query, results)
	return nil
}

func newMemRecallCmd() *cobra.Command {
	var r recallArgs
	cmd := &cobra.Command{
		Use:     "recall <query>",
		Short:   "Search Nomos Mneme memory using hybrid subword and lexical relevance",
		Long:    tui.HelpMemRecallLong,
		Example: tui.HelpMemRecallExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemRecall(args[0], r)
		},
	}
	cmd.Flags().StringVar(&r.topic, "topic", "", "Filter memories by topic")
	cmd.Flags().StringVar(&r.kind, "kind", "", "Filter memories by kind")
	cmd.Flags().StringVarP(&r.project, tui.FlagNameProject, "p", "", "Filter memories by repository ID")
	cmd.Flags().BoolVarP(&r.crossRepo, "cross-repo", "A", false, "Search across all repositories")
	cmd.Flags().IntVarP(&r.limit, tui.FlagNameLimit, "n", 5, "Maximum matches to return")
	cmd.Flags().BoolVar(&r.jsonOutput, tui.FlagNameJSON, false, "Output results in JSON format")
	return cmd
}

func renderRecallResults(query string, results []*mem.SearchResult) {
	fmt.Printf("%s %s\n",
		tui.RenderBrandTitle("🧠 Nomos Mneme Episodic Recall:"),
		tui.UiBoldValueStyle.Render(fmt.Sprintf("%q", query)))
	fmt.Println(tui.UiRule(tui.RuleNarrow))
	if len(results) == 0 {
		fmt.Printf("  %s\n", tui.UiMutedStyle.Render("No matching memories found."))
		fmt.Println(tui.UiRule(tui.RuleNarrow))
		return
	}
	for i, r := range results {
		fmt.Printf("  %s %s %s %s\n",
			tui.UiMutedStyle.Render(fmt.Sprintf("%d.", i+1)),
			tui.UiSuccessStyle.Render(fmt.Sprintf("[%.2f]", r.Score)),
			tui.UiCommandStyle.Render(fmt.Sprintf("#%d", r.Memory.ID)),
			tui.UiLabelStyle.Render(fmt.Sprintf("(%s · %s)", r.Memory.Kind, r.Memory.Topic)))
		fmt.Printf("     %s\n", tui.UiValueStyle.Render(r.Snippet))
	}
	fmt.Println(tui.UiRule(tui.RuleNarrow))
	fmt.Printf("%s\n", tui.UiFooterStyle.Render(fmt.Sprintf("Found %d relevant memory record(s)", len(results))))
}

type listArgs struct {
	taskID, kind, topic, project, status string
	listAll, jsonOutput                  bool
	limit                                int
}

func runMemList(out io.Writer, l listArgs) error {
	statusFilter, sErr := resolveMemListStatusFilter(l.status)
	if sErr != nil {
		return sErr
	}
	repoID := l.project
	if !l.listAll && repoID == "" {
		repoID = ResolveCurrentRepoID()
	}

	store, err := mem.NewStore(config.ResolveMnemeDBPath())
	if err != nil {
		return err
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// safe to continue
		}
	}()

	filter := mem.MemoryFilter{
		TaskID:           l.taskID,
		Kind:             mem.MemoryKind(l.kind),
		Topic:            l.topic,
		Status:           statusFilter,
		RepoID:           repoID,
		IncludeCrossRepo: l.listAll,
		Limit:            l.limit,
	}
	memories, err := store.Query(filter)
	if err != nil {
		return err
	}

	if l.jsonOutput {
		data, mErr := json.MarshalIndent(memories, "", "  ")
		if mErr != nil {
			return types.NewNomosError(types.ErrCodeCore, "failed marshaling json: %v", mErr)
		}
		fmt.Fprintln(out, string(data))
		return nil
	}

	renderMemoryList(out, memories)
	return nil
}

func newMemListCmd() *cobra.Command {
	var l listArgs
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List and inspect stored Nomos Mneme memories",
		Long:    tui.HelpMemListLong,
		Example: tui.HelpMemListExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemList(cmd.OutOrStdout(), l)
		},
	}
	cmd.Flags().StringVar(&l.taskID, "task", "", "Filter by task ID")
	cmd.Flags().StringVar(&l.kind, "kind", "", "Filter by kind")
	cmd.Flags().StringVar(&l.topic, "topic", "", "Filter by topic")
	cmd.Flags().StringVar(&l.status, "status", string(mem.StatusActive), "Filter by lifecycle status (active, superseded, tombstoned, or all)")
	cmd.Flags().StringVarP(&l.project, tui.FlagNameProject, "p", "", "Filter by repository ID")
	cmd.Flags().BoolVarP(&l.listAll, "all", "A", false, "Show memories across all repositories")
	cmd.Flags().IntVarP(&l.limit, tui.FlagNameLimit, "n", 20, "Maximum records to return")
	cmd.Flags().BoolVar(&l.jsonOutput, tui.FlagNameJSON, false, "Output results in JSON format")
	return cmd
}

func renderMemoryList(out io.Writer, memories []*mem.Memory) {
	fmt.Fprintf(out, "%s %s\n",
		tui.RenderBrandTitle("🧠 Nomos Mneme Memory Records"),
		tui.UiMutedStyle.Render(fmt.Sprintf("(%d found)", len(memories))))
	fmt.Fprintln(out, tui.UiRule(tui.RuleWide))
	if len(memories) == 0 {
		fmt.Fprintf(out, "  %s\n", tui.UiMutedStyle.Render("No matching memories found."))
		fmt.Fprintln(out, tui.UiRule(tui.RuleWide))
		return
	}
	for _, m := range memories {
		taskStr := m.TaskID
		if taskStr == "" {
			taskStr = "-"
		}
		repoTag := m.RepoID
		if repoTag == "" {
			repoTag = "global"
		}
		statusTag := string(m.Status)
		if statusTag == "" {
			statusTag = string(mem.StatusActive)
		}
		snippet := strings.ReplaceAll(m.Content, "\n", " ")
		if len(snippet) > 60 {
			snippet = snippet[:57] + "..."
		}
		fmt.Fprintf(out, "  %s %s %s %s %s %s: %s\n",
			tui.PadStyled(fmt.Sprintf("#%d", m.ID), 6, tui.UiCommandStyle),
			tui.PadStyled("["+string(m.Kind)+"]", 12, tui.UiIntentStyle),
			tui.PadStyled("["+statusTag+"]", 12, tui.UiMutedStyle),
			tui.PadStyled("["+repoTag+"]", 14, tui.UiSubstrateStyle),
			tui.PadStyled("["+taskStr+"]", 10, tui.TaskKeyStyleFor(taskStr)),
			tui.PadStyled("("+m.Topic+")", 22, tui.UiLabelStyle),
			tui.UiValueStyle.Render(snippet))
	}
	fmt.Fprintln(out, tui.UiRule(tui.RuleWide))
}

// ResolveCurrentRepoID executes deterministic inspectcmd verification logic (@intent task_lifecycle_cli_spec.md).
func ResolveCurrentRepoID() string {
	if repoRoot, err := workspace.FindRepoRoot(""); err == nil {
		return filepath.Base(filepath.Clean(repoRoot))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "nomos"
	}
	return filepath.Base(filepath.Clean(cwd))
}
