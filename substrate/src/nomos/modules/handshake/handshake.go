// Package handshake implements JIT session orientation telemetry, CapBAC tokens,
// and NCTE-100 prose validation enforced by compiled Go SSoT contracts.
package handshake

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mem"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/task"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/impact"
)

// DirectivePhase represents valid workspace phases for JIT directives (#DirectivePhase).
type DirectivePhase types.Phase

// HandshakeEvent represents JIT session orientation telemetry (#HandshakeEvent).
type HandshakeEvent struct {
	Seat             string            `json:"seat"`
	Repository       string            `json:"repository"`
	Plane            string            `json:"plane,omitempty"`
	Phase            types.Phase       `json:"phase"`
	ActiveTask       string            `json:"active_task,omitempty"`
	Assignee         string            `json:"assignee,omitempty"`
	TokenID          string            `json:"token_id,omitempty"`
	StepBudget       int               `json:"step_budget,omitempty"`
	TokensUsed       int               `json:"tokens_used,omitempty"`
	DatabasesHealthy bool              `json:"databases_healthy"`
	GateCount        int               `json:"gate_count"`
	ReadyWavefront   []string          `json:"ready_wavefront"`
	Directives           []string                 `json:"directives,omitempty"`
	LearnedPitfalls      []string                 `json:"learned_pitfalls,omitempty"`
	VerifiedPatterns     []string                 `json:"verified_patterns,omitempty"`
	GraduationCandidates []string                 `json:"graduation_candidates,omitempty"`
	ImpactSummary        []string                 `json:"impact_summary,omitempty"`
	StaleObligations     []task.StaleSpecObligation `json:"stale_obligations,omitempty"`
	StrictMode           bool                     `json:"strict_mode"`
	BrownfieldTopology   *BrownfieldTopologyBrief `json:"brownfield_topology,omitempty"`
	NextActions          []types.ActionAffordance `json:"next_actions,omitempty"`
	DeprecatedTerms      map[string]string        `json:"deprecated_terms,omitempty"`
	Timestamp            string                   `json:"timestamp,omitempty"`
}

// DirectiveFrontmatter defines the authoritative schema for JIT directives (#DirectiveFrontmatter).
type DirectiveFrontmatter struct {
	Title          string           `json:"title" yaml:"title"`
	Description    string           `json:"description,omitempty" yaml:"description,omitempty"`
	Authors        []string         `json:"authors" yaml:"authors"`
	Classification string           `json:"classification" yaml:"classification"`
	Date           string           `json:"date" yaml:"date"`
	LastUpdated    string           `json:"last_updated" yaml:"last_updated"`
	Plane          string           `json:"plane" yaml:"plane"`
	Status         string           `json:"status" yaml:"status"`
	Type           string           `json:"type" yaml:"type"`
	Phases         []DirectivePhase `json:"phases" yaml:"phases"`
	Category       string           `json:"category,omitempty" yaml:"category,omitempty"`
	Priority       string           `json:"priority,omitempty" yaml:"priority,omitempty"`
	Items          []string         `json:"items,omitempty" yaml:"items,omitempty"`
	Summary        string           `json:"summary,omitempty" yaml:"summary,omitempty"`
}

// SynthesizeDynamicHandshake inspects live workspace state, databases, and quality gates.
// @intent INT-840 #HandshakeEvent
func SynthesizeDynamicHandshake(repoRoot string) (*HandshakeEvent, error) {
	treeRoot, primaryRoot, err := resolveHandshakeRoots(repoRoot)
	if err != nil {
		return nil, err
	}

	seat := resolveHandshakeSeat()
	repo := filepath.Base(primaryRoot)

	_, activeTask := detectBranchAndTask(treeRoot)
	healthy := checkDatabasesHealth()
	gateCount := verify.RegisteredGateCount()

	phase, plane, leaseID, stepBudget, tokensUsed := queryTaskAndLease(activeTask)
	wavefront := queryReadyWavefront(task.ResolveRepoProject(primaryRoot))
	directives := loadPhaseDirectives(primaryRoot, phase)
	modifiedFiles, impactSummary := computeActiveTaskImpact(treeRoot, activeTask)
	pitfalls, patterns, graduations := recallMnemeHandshakeContext(primaryRoot, activeTask, modifiedFiles)
	strictMode, bfTopology := resolveHandshakeGovernance(treeRoot, primaryRoot)
	staleObligations := resolveHandshakeStaleObligations(primaryRoot)

	wtRoot := ""
	if activeTask != "" && strings.Contains(filepath.ToSlash(treeRoot), "/worktrees/") {
		wtRoot = treeRoot
	}
	nextActions := types.BuildPhaseNextActions(phase, activeTask, wtRoot, directives, wavefront)

	event := &HandshakeEvent{
		Seat:                 seat,
		Repository:           repo,
		Plane:                plane,
		Phase:                phase,
		ActiveTask:           activeTask,
		TokenID:              leaseID,
		StepBudget:           stepBudget,
		TokensUsed:           tokensUsed,
		DatabasesHealthy:     healthy,
		GateCount:            gateCount,
		ReadyWavefront:       wavefront,
		Directives:           directives,
		LearnedPitfalls:      pitfalls,
		VerifiedPatterns:     patterns,
		GraduationCandidates: graduations,
		ImpactSummary:        impactSummary,
		StaleObligations:     staleObligations,
		StrictMode:           strictMode,
		BrownfieldTopology:   bfTopology,
		NextActions:          nextActions,
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
	}

	if valErr := ValidateHandshakeEvent(event); valErr != nil {
		return nil, valErr
	}
	return event, nil
}

func resolveHandshakeSeat() string {
	if seat := os.Getenv("NOMOS_SEAT"); seat != "" {
		return seat
	}
	return "workstation-local"
}

func resolveHandshakeStaleObligations(primaryRoot string) []task.StaleSpecObligation {
	stale, err := task.ComputeStaleSpecObligations(primaryRoot, "")
	if err != nil {
		return nil
	}
	return stale
}

func resolveHandshakeRoots(repoRoot string) (string, string, error) {
	if repoRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", types.NewNomosError(types.ErrCodeBoundary, "failed resolving working directory: %v", err)
		}
		repoRoot = cwd
	}
	treeRoot := repoRoot
	if tr, tErr := workspace.FindWorkingTreeRoot(repoRoot); tErr == nil {
		treeRoot = tr
	}
	primaryRoot := treeRoot
	if pr, pErr := workspace.FindRepoRoot(repoRoot); pErr == nil {
		primaryRoot = pr
	}
	return treeRoot, primaryRoot, nil
}

func computeActiveTaskImpact(treeRoot, activeTask string) ([]string, []string) {
	if activeTask == "" {
		return nil, nil
	}
	modified := impact.ListModifiedWorktreeFiles(treeRoot, activeTask)
	return modified, impact.ComputeWorkspaceImpact(treeRoot, modified)
}

var scopePathHintRe = regexp.MustCompile(`[a-zA-Z0-9_.-]+(?:/[a-zA-Z0-9_.-]+)+\.[a-zA-Z0-9]+`)

func recallMnemeHandshakeContext(primaryRoot, activeTask string, modifiedFiles []string) ([]string, []string, []string) {
	var pitfalls, patterns []string
	taskTitle, scopeHints := resolveActiveTaskContext(activeTask)
	if strings.TrimSpace(activeTask) == "" || len(modifiedFiles) == 0 {
		pitfalls = mem.RecallRepoPitfallsReadOnly(primaryRoot, scopeHints, 3)
		patterns = mem.RecallRepoPatternsReadOnly(primaryRoot, taskTitle, scopeHints, 3)
	} else {
		pitfalls = mem.RecallRepoPitfallsForTask(primaryRoot, activeTask, modifiedFiles, 3)
		patterns = mem.RecallRepoPatternsForTask(primaryRoot, activeTask, taskTitle, modifiedFiles, 3)
	}
	graduations := mem.RecallGraduationCandidates(primaryRoot, 3)
	return pitfalls, patterns, graduations
}

func resolveActiveTaskContext(activeTask string) (string, []string) {
	if strings.TrimSpace(activeTask) == "" {
		return "", nil
	}
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		return "", nil
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()
	if rec, tErr := store.GetTask(activeTask); tErr == nil && rec != nil {
		hints := scopePathHintRe.FindAllString(rec.Title+"\n"+rec.Description, 8)
		return rec.Title, hints
	}
	return "", nil
}

func detectBranchAndTask(repoRoot string) (string, string) {
	cmd := workspace.SafeGitCommand(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "main", ""
	}
	branch := strings.TrimSpace(string(out))
	if strings.HasPrefix(branch, "feature/") {
		taskKey := strings.TrimPrefix(branch, "feature/")
		return branch, taskKey
	}
	return branch, ""
}

func checkDatabasesHealth() bool {
	globalDB := config.ResolveGlobalDBPath()
	db1, err := storage.OpenDB(globalDB)
	if err != nil {
		return false
	}
	if pErr := db1.Ping(); pErr != nil {
		if cErr := db1.Close(); cErr != nil {
			// closed
		}
		return false
	}
	if cErr := db1.Close(); cErr != nil {
		// closed
	}

	brainDB := config.ResolveMnemeDBPath()
	memStore, err := mem.NewStore(brainDB)
	if err != nil {
		return false
	}
	if cErr := memStore.Close(); cErr != nil {
		// closed
	}
	return true
}

func queryTaskAndLease(taskKey string) (types.Phase, string, string, int, int) {
	if taskKey == "" {
		return types.PhaseIdle, "substrate", "", 50, 0
	}

	phase := types.PhasePlan
	plane := "substrate"
	if strings.HasPrefix(taskKey, "INT-") {
		plane = "intent"
	}

	var taskStatusPhase types.Phase
	store, err := task.NewDefaultSQLiteTaskStore()
	if err == nil {
		defer func() {
			if cErr := store.Close(); cErr != nil {
				// closed
			}
		}()
		if rec, tErr := store.GetTask(taskKey); tErr == nil && rec != nil {
			if rec.Plane != "" {
				plane = string(rec.Plane)
			}
			if types.IsValidPhase(types.Phase(rec.Status)) {
				taskStatusPhase = types.Phase(rec.Status)
				phase = taskStatusPhase
			}
		}
	}

	db, err := storage.OpenDB(config.ResolveGlobalDBPath())
	if err != nil {
		return phase, plane, "", 50, 0
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			// closed
		}
	}()

	leaseID, lPhase, budget, used, lErr := storage.GetActiveLease(db, taskKey)
	if lErr == nil {
		if taskStatusPhase != "" {
			phase = taskStatusPhase
		} else if types.IsValidPhase(types.Phase(lPhase)) {
			phase = types.Phase(lPhase)
		}
		return phase, plane, leaseID, budget, used
	}

	return phase, plane, "", 50, 0
}

func queryReadyWavefront(activeProject string) []string {
	store, err := task.NewDefaultSQLiteTaskStore()
	if err != nil {
		return []string{}
	}
	defer func() {
		if cErr := store.Close(); cErr != nil {
			// closed
		}
	}()

	candidates, err := store.GetWavefront()
	if err != nil {
		return []string{}
	}

	var wavefront []string
	for _, cand := range candidates {
		if task.IsInProgressStatus(cand.Status) || !task.MatchesProject(cand.Project, activeProject, true) {
			continue
		}
		wavefront = append(wavefront, cand.Key)
		if len(wavefront) >= 5 {
			break
		}
	}
	return wavefront
}



// ValidateHandshakeEvent asserts that HandshakeEvent satisfies structural invariants.
func ValidateHandshakeEvent(event *HandshakeEvent) error {
	if event == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "handshake event cannot be nil")
	}
	if strings.TrimSpace(event.Seat) == "" {
		return types.NewNomosError(types.ErrCodeValidationFailed, "seat cannot be empty")
	}
	if strings.TrimSpace(event.Repository) == "" {
		return types.NewNomosError(types.ErrCodeValidationFailed, "repository cannot be empty")
	}
	if !types.IsValidPhase(event.Phase) {
		return types.NewNomosError(types.ErrCodePhaseViolation, "invalid phase %q", event.Phase)
	}
	return nil
}
