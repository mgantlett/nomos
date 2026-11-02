// Package cli implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package cli

const (
	flagNameJSON    = "json"
	flagNameProject = "project"
	flagNamePlane   = "plane"
	flagNameTier    = "tier"
	flagNameStatus  = "status"
	flagNamePending = "pending"
	flagNameLimit   = "limit"
)

const (
	helpRootExample = `  nomos init
  nomos status
  nomos task dag
  nomos verify --all`

	helpInitLong = `Bootstrap or synchronize a repository with Nomos governance contracts:
scaffolds nomos.yaml, synchronizes managed protocol blocks in AGENTS.md and CLAUDE.md,
chains ambient lifecycle hooks, and installs Git pre-commit/commit-msg/pre-push guards.`
	helpInitExample = `  nomos init
  nomos init --dir ./my-service`

	helpStatusExample = `  nomos status
  nomos status --json`

	helpVerifyExample = `  nomos verify
  nomos verify --all --strict
  nomos verify --fast-file substrate/cli/root.go
  nomos verify --impact`

	helpVerifyProofExample = `  nomos verify proof
  nomos verify proof . --json`

	helpPhysicsLong = `Compute Software Physics mass and real-time collision vectors across active
transient task worktrees. Inspects touched files, AST symbol exports, and package
fan-in/fan-out to classify concurrent worktree proximity as GREEN, YELLOW, or RED.`
	helpPhysicsExample = `  nomos physics
  nomos physics --json`

	helpHandshakeExample = `  nomos handshake
  nomos handshake --json`

	helpHookExample = `  nomos hook pre-tool < payload.json
  nomos hook post-tool --hermetic-dir /workspace < payload.json`

	helpHookSessionStartLong = `Ingest a JSON SessionStart hook payload from stdin, ensure repository Git
defense hooks are active, and emit a deterministic JIT workspace orientation banner.`
	helpHookSessionStartExample = `  echo '{"cwd":"."}' | nomos hook session-start`

	helpHookPreToolLong = `Ingest a JSON PreToolUse payload from stdin and enforce Law 3 worktree boundary
isolation, unexpanded shell variable hygiene, Law 7 command safety, and the 3-strike
identical-failure circuit-breaker before mutating tools execute.`
	helpHookPreToolExample = `  echo '{"tool_name":"Write","tool_input":{"file_path":"main.go"}}' | nomos hook pre-tool`

	helpHookPostToolLong = `Ingest a JSON PostToolUse payload from stdin, remain 100% silent on non-edit
channels to preserve native OS error provenance, and run zero-footprint in-memory
syntax and budget radar checks on modified source files.`
	helpHookPostToolExample = `  echo '{"tool_name":"Edit","tool_input":{"file_path":"worktrees/repo-SUB-1/app.go"}}' | nomos hook post-tool`

	helpDirectiveExample = `  nomos directive
  nomos directive plan_03_nomos_controlled_technical_english.md
  nomos directive --lexicon --domain steering
  nomos directive --term sandbox`

	helpCapabilitiesExample = `  nomos capabilities
  nomos capabilities --type gate
  nomos capabilities plane_separation --json`

	helpMemExample = `  nomos mem recall "worktree"
  nomos mem list --limit 10
  nomos mem save --kind reflection --topic cli "Keep leaf commands self-documenting"`

	helpMemSaveLong = `Persist an advisory reflection, architectural axiom, or decision record into
Nomos Mneme SQLite storage partitioned to the active repository scope.`
	helpMemSaveExample = `  nomos mem save "Prefer modular helper files under 300 LOC"
  nomos mem save --kind decision --topic verify --project nomos "Audit CLI help in Cell 6.2"`

	helpMemRecallLong = `Query Nomos Mneme episodic and statutory memory records using hybrid lexical
and semantic vector similarity scoped to the active repository (or --cross-repo).`
	helpMemRecallExample = `  nomos mem recall "plane separation"
  nomos mem recall "worktree" --cross-repo --limit 10 --json`

	helpMemListLong = `Inspect stored Nomos Mneme reflections, axioms, decisions, learned pitfalls,
and verified patterns filtered by repository, task key, memory kind, or topic.`
	helpMemListExample = `  nomos mem list
  nomos mem list --kind pattern --limit 10
  nomos mem list --all --json`

	helpSessionExample = `  nomos session list
  nomos session show
  nomos session name "CLI Surface Hardening"`

	helpSessionNameExample = `  nomos session name "Wave A Hardening"
  nomos session name --id 12579ec0 "Audit Probes" --json`

	helpSessionListLong = `Discover and list active or historical Tier 1 coding agent conversation sessions
across supported IDE and CLI providers with status and last-active timestamps.`
	helpSessionListExample = `  nomos session list
  nomos session list --json`

	helpSessionVisualLong = `Synthesize a deterministic Mermaid topology graph linking active Tier 1 agent
sessions to workspace branches, tasks, and mission states.`
	helpSessionVisualExample = `  nomos session visual
  nomos session visual --json`

	helpSessionShowLong = `Inspect turn count, tool invocations, causal trajectory progress, and Sentinel
integrity scoring for a specific or active Tier 1 agent conversation session.`
	helpSessionShowExample = `  nomos session show
  nomos session show 12579ec0 --json`

	helpIssueExample = `  nomos issue --dry-run "CLI flag mismatch"
  nomos issue "Verify gate timeout" --body "Observed on large monorepo"`

	helpVersionLong = `Display the compiled Nomos Open-Core substrate version, target OS/architecture,
and Go toolchain provenance.`
	helpVersionExample = `  nomos version
  nomos version --json`
)
