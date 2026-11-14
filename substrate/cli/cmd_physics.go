// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mtpo"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/astdebt"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
	"github.com/spf13/cobra"
)

// PhysicsReport models active worktree hulls and pairwise collision vectors (#WorktreeHull).
type PhysicsReport struct {
	RepoRoot   string             `json:"repo_root"`
	Hulls      []WorktreeHull     `json:"hulls"`
	Collisions []*CollisionVector `json:"collisions"`
	MaxThreat  ThreatLevel        `json:"max_threat"`
}

// NewPhysicsCmd constructs the canonical 'nomos physics' command.
// @intent software_physics_engine_spec.md
func NewPhysicsCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "physics",
		Short:   "Inspect active worktree AST mass and real-time symbol/file collision vectors",
		Long:    tui.HelpPhysicsLong,
		Example: tui.HelpPhysicsExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot, err := workspace.FindRepoRoot("")
			if err != nil || repoRoot == "" {
				repoRoot = "."
			}
			rep := CollectWorkspacePhysics(repoRoot)
			out := cmd.OutOrStdout()
			if jsonOutput {
				data, mErr := json.MarshalIndent(rep, "", "  ")
				if mErr != nil {
					return types.NewNomosError(types.ErrCodeCore, "failed marshaling physics report: %v", mErr)
				}
				fmt.Fprintln(out, string(data))
				return nil
			}
			renderPhysicsReport(out, rep)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, tui.FlagNameJSON, false, "Emit software physics report as structured JSON")
	return cmd
}

// CollectWorkspacePhysics discovers active transient worktrees and evaluates collision proximity.
func CollectWorkspacePhysics(repoRoot string) *PhysicsReport {
	rep := &PhysicsReport{
		RepoRoot:   repoRoot,
		Hulls:      []WorktreeHull{},
		Collisions: []*CollisionVector{},
		MaxThreat:  ThreatGreen,
	}

	wtRoot := filepath.Join(repoRoot, "worktrees")
	entries, err := os.ReadDir(wtRoot)
	if err != nil {
		return rep
	}
	mtpoByKey := loadActiveMTPOMap(repoRoot)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wtPath := filepath.Join(wtRoot, e.Name())
		hull := buildWorktreeHull(wtPath, e.Name())
		if ledger, ok := mtpoByKey[hull.TaskKey]; ok {
			hull.MTPOSeq = ledger.Seq
			hull.MutatedExports = unionUniqueStrings(hull.MutatedExports, ledger.WriteSymbols)
			hull.ReadSymbols = unionUniqueStrings(hull.ReadSymbols, ledger.ReadSymbols)
			hull.ReferencedImports = unionUniqueStrings(hull.ReferencedImports, ledger.ReferencedImports)
		}
		rep.Hulls = append(rep.Hulls, hull)
	}

	for i := 0; i < len(rep.Hulls); i++ {
		for j := i + 1; j < len(rep.Hulls); j++ {
			col := DetectCollision(&rep.Hulls[i], &rep.Hulls[j])
			if col == nil {
				continue
			}
			rep.Collisions = append(rep.Collisions, col)
			if col.ThreatLevel == ThreatRed {
				rep.MaxThreat = ThreatRed
			} else if col.ThreatLevel == ThreatYellow && rep.MaxThreat == ThreatGreen {
				rep.MaxThreat = ThreatYellow
			}
		}
	}
	return rep
}

func buildWorktreeHull(wtPath, dirName string) WorktreeHull {
	taskKey := extractTaskKeyFromDirName(dirName)
	touched := impact.ListModifiedWorktreeFiles(wtPath, taskKey)
	sort.Strings(touched)

	exportsMap := map[string]bool{}
	importsMap := map[string]bool{}
	totalMass := 0.0

	for _, rel := range touched {
		fullPath := filepath.Join(wtPath, rel)
		fm := ComputeGoFileMass(fullPath, rel, exportsMap, importsMap)
		totalMass += fm.TotalMass
	}

	return WorktreeHull{
		TaskKey:           taskKey,
		WorktreePath:      wtPath,
		Phase:             "EDIT",
		MutatedExports:    tui.SortedMapKeys(exportsMap),
		ReferencedImports: tui.SortedMapKeys(importsMap),
		TouchedFiles:      touched,
		TotalMass:         totalMass,
	}
}

// ComputeGoFileMass calculates structural AST mass and extracts exported symbols for a file.
func ComputeGoFileMass(fullPath, relPath string, exports, imports map[string]bool) FileMass {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return FileMass{Path: relPath}
	}
	loc := strings.Count(string(data), "\n") + 1
	fm := FileMass{Path: relPath, LOC: loc, TotalMass: float64(loc)}
	if !strings.HasSuffix(fullPath, ".go") {
		return fm
	}

	fset := token.NewFileSet()
	node, pErr := parser.ParseFile(fset, fullPath, data, 0)
	if pErr != nil {
		return fm
	}

	for _, imp := range node.Imports {
		imports[strings.Trim(imp.Path.Value, `"`)] = true
		fm.FanOut++
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if n != nil {
			fm.ASTNodes++
		}
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.IsExported() {
			exports[node.Name.Name+"."+fn.Name.Name] = true
			fm.CyclomaticComplexity += astdebt.CalculateCyclomaticComplexity(fn)
		}
		return true
	})
	fm.TotalMass = float64(fm.LOC) + float64(fm.CyclomaticComplexity*4) + float64(fm.FanOut*2)
	return fm
}

func renderPhysicsReport(out io.Writer, rep *PhysicsReport) {
	fmt.Fprintln(out, tui.RenderBrandTitle("🧭 Nomos Software Physics & Worktree Collision Radar"))
	fmt.Fprintln(out, tui.UiRule(tui.RuleStandard))
	fmt.Fprintln(out, tui.KvLine("Active Worktrees", 18, tui.UiBoldValueStyle.Render(fmt.Sprintf("%d", len(rep.Hulls)))))
	fmt.Fprintln(out, tui.KvLine("Collision Threat", 18, tui.StyleSemanticBadge(string(rep.MaxThreat))))
	for _, h := range rep.Hulls {
		fmt.Fprintf(out, "  • [%s] seq=#%d files=%d exports=%d mass=%.1f (%s)\n",
			tui.StyleTaskKey(h.TaskKey), h.MTPOSeq, len(h.TouchedFiles), len(h.MutatedExports), h.TotalMass, tui.UiMutedStyle.Render(h.WorktreePath))
	}
	for _, c := range rep.Collisions {
		if c.ThreatLevel != ThreatGreen {
			fmt.Fprintf(out, "  ⚠️  [%s] %s <-> %s | files=%v symbols=%v\n",
				c.ThreatLevel, filepath.Base(c.WorktreeA), filepath.Base(c.WorktreeB), c.IntersectedFiles, c.IntersectedSymbols)
		}
	}
	fmt.Fprintln(out, tui.UiRule(tui.RuleStandard))
	tui.RenderNextActions(out, []types.ActionAffordance{
		{Command: "nomos status", Summary: "Inspect workspace lifecycle state and active leases"},
		{Command: "nomos verify --impact", Summary: "Inspect 1-hop AST blast-radius map for modified files"},
	})
}

func loadActiveMTPOMap(repoRoot string) map[string]mtpo.MTPOLedgerEntry {
	out := make(map[string]mtpo.MTPOLedgerEntry)
	entries, err := mtpo.ListActiveEntries("", repoRoot)
	if err != nil {
		return out
	}
	for _, e := range entries {
		out[e.TaskKey] = e
	}
	return out
}
