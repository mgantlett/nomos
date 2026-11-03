// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
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
		Short: "Nomos Mneme episodic and semantic memory",
		Long: `Nomos Mneme episodic memory subsystem. Records reflections,
axioms, and architectural decisions with deterministic vector recall.`,
		Example: helpMemExample,
	}

	cmd.AddCommand(newMemSaveCmd())
	cmd.AddCommand(newMemRecallCmd())
	cmd.AddCommand(newMemListCmd())
	cmd.AddCommand(newMemDismissCmd())
	return cmd
}

func newMemSaveCmd() *cobra.Command {
	var kind, topic, project string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "save <content>",
		Short:   "Save knowledge reflection or lesson into Nomos Mneme",
		Long:    helpMemSaveLong,
		Example: helpMemSaveExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemSave(cmd.OutOrStdout(), args[0], kind, topic, project, jsonOutput)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", string(mem.KindReflection), "Advisory kind of memory (reflection, axiom, decision)")
	cmd.Flags().StringVar(&topic, "topic", "General", "Topic identifier")
	cmd.Flags().StringVarP(&project, flagNameProject, "p", "", "Repository identifier (defaults to active repo)")
	cmd.Flags().BoolVar(&jsonOutput, flagNameJSON, false, "Emit saved memory record as structured JSON")
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
		repoID = resolveCurrentRepoID()
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
		uiSuccessStyle.Render("Nomos Mneme memory"), m.ID,
		uiSubstrateStyle.Render(m.RepoID), uiValueStyle.Render(m.Topic),
		uiIntentStyle.Render(string(m.Kind)))
	renderNextActions(os.Stdout, []types.ActionAffordance{
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
		repoID = resolveCurrentRepoID()
	}

	opts := mem.SearchOptions{
		Limit:            r.limit,
		Kind:             mem.MemoryKind(r.kind),
		Topic:            r.topic,
		RepoID:           repoID,
		IncludeCrossRepo: r.crossRepo,
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
		Short:   "Search Nomos Mneme memory using semantic and lexical relevance",
		Long:    helpMemRecallLong,
		Example: helpMemRecallExample,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemRecall(args[0], r)
		},
	}
	cmd.Flags().StringVar(&r.topic, "topic", "", "Filter memories by topic")
	cmd.Flags().StringVar(&r.kind, "kind", "", "Filter memories by kind")
	cmd.Flags().StringVarP(&r.project, flagNameProject, "p", "", "Filter memories by repository ID")
	cmd.Flags().BoolVarP(&r.crossRepo, "cross-repo", "A", false, "Search across all repositories")
	cmd.Flags().IntVarP(&r.limit, flagNameLimit, "n", 5, "Maximum matches to return")
	cmd.Flags().BoolVar(&r.jsonOutput, flagNameJSON, false, "Output results in JSON format")
	return cmd
}

func renderRecallResults(query string, results []*mem.SearchResult) {
	fmt.Printf("%s %s\n",
		renderBrandTitle("🧠 Nomos Mneme Episodic Recall:"),
		uiBoldValueStyle.Render(fmt.Sprintf("%q", query)))
	fmt.Println(uiRule(ruleNarrow))
	if len(results) == 0 {
		fmt.Printf("  %s\n", uiMutedStyle.Render("No matching memories found."))
		fmt.Println(uiRule(ruleNarrow))
		return
	}
	for i, r := range results {
		fmt.Printf("  %s %s %s %s\n",
			uiMutedStyle.Render(fmt.Sprintf("%d.", i+1)),
			uiSuccessStyle.Render(fmt.Sprintf("[%.2f]", r.Score)),
			uiCommandStyle.Render(fmt.Sprintf("#%d", r.Memory.ID)),
			uiLabelStyle.Render(fmt.Sprintf("(%s · %s)", r.Memory.Kind, r.Memory.Topic)))
		fmt.Printf("     %s\n", uiValueStyle.Render(r.Snippet))
	}
	fmt.Println(uiRule(ruleNarrow))
	fmt.Printf("%s\n", uiFooterStyle.Render(fmt.Sprintf("Found %d relevant memory record(s)", len(results))))
}

type listArgs struct {
	taskID, kind, topic, project string
	listAll, jsonOutput          bool
	limit                        int
}

func runMemList(l listArgs) error {
	repoID := l.project
	if !l.listAll && repoID == "" {
		repoID = resolveCurrentRepoID()
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
		fmt.Println(string(data))
		return nil
	}

	renderMemoryList(memories)
	return nil
}

func newMemListCmd() *cobra.Command {
	var l listArgs
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List and inspect stored Nomos Mneme memories",
		Long:    helpMemListLong,
		Example: helpMemListExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMemList(l)
		},
	}
	cmd.Flags().StringVar(&l.taskID, "task", "", "Filter by task ID")
	cmd.Flags().StringVar(&l.kind, "kind", "", "Filter by kind")
	cmd.Flags().StringVar(&l.topic, "topic", "", "Filter by topic")
	cmd.Flags().StringVarP(&l.project, flagNameProject, "p", "", "Filter by repository ID")
	cmd.Flags().BoolVarP(&l.listAll, "all", "A", false, "Show memories across all repositories")
	cmd.Flags().IntVarP(&l.limit, flagNameLimit, "n", 20, "Maximum records to return")
	cmd.Flags().BoolVar(&l.jsonOutput, flagNameJSON, false, "Output results in JSON format")
	return cmd
}

func renderMemoryList(memories []*mem.Memory) {
	fmt.Printf("%s %s\n",
		renderBrandTitle("🧠 Nomos Mneme Memory Records"),
		uiMutedStyle.Render(fmt.Sprintf("(%d found)", len(memories))))
	fmt.Println(uiRule(ruleWide))
	if len(memories) == 0 {
		fmt.Printf("  %s\n", uiMutedStyle.Render("No matching memories found."))
		fmt.Println(uiRule(ruleWide))
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
		snippet := strings.ReplaceAll(m.Content, "\n", " ")
		if len(snippet) > 60 {
			snippet = snippet[:57] + "..."
		}
		fmt.Printf("  %s %s %s %s %s: %s\n",
			padStyled(fmt.Sprintf("#%d", m.ID), 6, uiCommandStyle),
			padStyled("["+string(m.Kind)+"]", 12, uiIntentStyle),
			padStyled("["+repoTag+"]", 14, uiSubstrateStyle),
			padStyled("["+taskStr+"]", 10, taskKeyStyleFor(taskStr)),
			padStyled("("+m.Topic+")", 22, uiLabelStyle),
			uiValueStyle.Render(snippet))
	}
	fmt.Println(uiRule(ruleWide))
}

func resolveCurrentRepoID() string {
	if repoRoot, err := workspace.FindRepoRoot(""); err == nil {
		return filepath.Base(filepath.Clean(repoRoot))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "nomos"
	}
	return filepath.Base(filepath.Clean(cwd))
}
