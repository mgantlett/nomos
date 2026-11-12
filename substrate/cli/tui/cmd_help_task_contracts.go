// Package tui implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package tui

const (
	HelpTaskExample = `  nomos task dag
  nomos task list --pending
  nomos task chore "Fix parser edge case"
  nomos task sync SUB-101 -F .tmp/walkthrough-SUB-101.md`

	HelpTaskListLong = `Query and filter Living Ledger task records stored in pure-Go SQLite persistence.
Defaults to the active repository project unless --project or -A/--all-projects is passed.
Supports filtering by lifecycle status, pending state, plane (intent or substrate),
cognitive tier (T1 or T2), and row limit. For dependency-ready unblocked work, see
'nomos task dag'; to inspect a single card, run 'nomos task show <KEY>'.`
	HelpTaskListExample = `  nomos task list --pending
  nomos task list --status TODO,EDIT --plane substrate --limit 20
  nomos task list --all-projects --json`

	HelpTaskDAGLong = `Compute the causal DAG wavefront of unblocked Living Ledger tasks whose upstream
dependencies are satisfied (DONE), scoped to the active repository project by default.`
	HelpTaskDAGExample = `  nomos task dag
  nomos task dag --plane substrate --limit 10
  nomos task dag --all-projects --json`

	HelpTaskStatusLong = `Display active workspace lifecycle phase (IDLE, PLAN, EDIT, REVIEW, SYNC, LEARN),
bound task key, CapBAC capability token budget consumption, and HATEOAS Next Actions.`
	HelpTaskStatusExample = `  nomos task status
  nomos task status --json`

	HelpTaskShowLong = `Inspect the complete Living Ledger 3-section task card, plane metadata, bilateral
twin linkage, upstream/downstream causal dependencies, and active worktree path.`
	HelpTaskShowExample = `  nomos task show SUB-101
  nomos task show INT-88 --json`

	HelpTaskCreateLong = `Allocate and persist a durable Living Ledger task card (INT-<N> or SUB-<N>) backed
by the mandatory 3-section contract (Scope & Context, Acceptance Criteria, Verification).`
	HelpTaskCreateExample = `  nomos task create "Add AST check" --plane substrate --scope "Add check in verify/" --ac "Pass unit tests" --verify "nomos verify"
  nomos task create "Author spec" --plane intent -F .tmp/card.md`

	HelpTaskPairLong = `Atomically allocate and bind twinned INT-<N> (Intent) and SUB-<N> (Substrate)
Living Ledger task cards so specification and implementation evolve in lockstep.`
	HelpTaskPairExample = `  nomos task pair "Add schema gate" --int-scope "Document contract spec" --sub-scope "Implement Go gate" --ac "42/42 gates pass" --verify "nomos verify"`

	HelpTaskChoreLong = `Provision a single-turn fast-lane task (burden 1) directly into an isolated git
worktree in EDIT phase with an active CapBAC lease and zero card boilerplate.`
	HelpTaskChoreExample = `  nomos task chore "Tighten CLI error message"
  nomos task chore "Fix typo in architecture spec" --plane intent`

	HelpTaskProjectLong = `Assign or reconcile the repository project identifier on one or more existing
Living Ledger task keys through canonical SQLite encapsulation (Law 1).`
	HelpTaskProjectExample = `  nomos task project nomos SUB-101 SUB-102
  nomos task project my-service INT-40,SUB-85`

	HelpTaskStartLong = `Provision a hermetic git worktree (worktrees/<repo>-<KEY>) branched from clean
trunk, mint an active CapBAC capability lease, and transition into PLAN or EDIT phase.`
	HelpTaskStartExample = `  nomos task start SUB-101 --edit
  nomos task start INT-42`

	HelpTaskEditLong = `Transition an existing task worktree and active CapBAC lease from PLAN or REVIEW
into EDIT phase so source mutations can proceed inside the isolated worktree.`
	HelpTaskEditExample = `  nomos task edit SUB-101`

	HelpTaskCommitLong = `Stage worktree modifications and create a Golden Conventional Commit formatted
with a canonical [KEY] type(scope) subject, flowing rationale prose, and RFC 822 Task/Twin Git trailers.`
	HelpTaskCommitExample = `  nomos task commit SUB-101 -m "feat(cli): add status filter to task list"
  nomos task commit -F .tmp/walkthrough-SUB-101.md`

	HelpTaskReviewLong = `Transition an active task worktree into REVIEW phase and evaluate all 42
Definition of Done quality gates across the 7x3 Canonical Invariant Suite.`
	HelpTaskReviewExample = `  nomos task review SUB-101`

	HelpTaskSyncLong = `Execute an atomic Two-Phase Commit (2PC) release: verify all 42 DoD quality gates,
validate the walkthrough summary, fast-forward merge onto trunk, seal a cryptographic
SHA-256 Git Note receipt, distill verified patterns into Mneme, and prune the worktree.`
	HelpTaskSyncExample = `  nomos task sync SUB-101 -m "Tighten boundary validation and add unit tests"
  nomos task sync SUB-101 -F .tmp/walkthrough-SUB-101.md`

	HelpTaskAbortLong = `Abort an in-flight task, revoke its CapBAC capability lease, remove its transient
worktree and unmerged feature branch, and reset workspace phase to IDLE.`
	HelpTaskAbortExample = `  nomos task abort SUB-101`

	HelpTaskPruneLong = `Sweep orphaned worktrees/* directories lacking active leases, delete merged
feature/* branches, and clean stale walkthrough artifacts safely.`
	HelpTaskPruneExample = `  nomos task prune --dry-run
  nomos task prune --json`
)
