// Package tui implements Tier 1 agent adapters, software physics telemetry,
// release readiness, upgrades, and CLI command dispatch contracts.
package tui

const (
	FlagNameJSON        = "json"
	FlagNameProject     = "project"
	FlagNameAllProjects = "all-projects"
	FlagNamePlane       = "plane"
	FlagNameTier        = "tier"
	FlagNameStatus      = "status"
	FlagNamePending     = "pending"
	FlagNameLimit       = "limit"
)

const (
	HelpRootExample = `  nomos xray
  nomos init --baseline
  nomos status
  nomos task dag
  nomos verify --all`

	HelpInitLong = `Bootstrap or synchronize a repository with Nomos governance contracts:
scaffolds nomos.yaml, synchronizes the managed protocol block in AGENTS.md and @AGENTS.md import in CLAUDE.md,
chains ambient lifecycle hooks, installs Git pre-commit/commit-msg/pre-push guards, and optionally snapshots Day-0 structural debt bounds (--baseline).`
	HelpInitExample = `  nomos init
  nomos init --baseline
  nomos init --dir ./my-service --baseline`

	HelpXrayLong = `Execute a zero-init, read-only structural debt and AI-hangover scan across 11 languages.
Evaluates Type-2 structural clones, >400 LOC monoliths, oversized/complex functions, empty stubs,
skipped tests, and CWE sinks in under 2 seconds without writing files or touching SQLite.`
	HelpXrayExample = `  nomos xray
  nomos xray ./legacy-service --markdown
  nomos xray --top 15 --json`

	HelpStatusExample = `  nomos status
  nomos status --all-projects
  nomos status --json`

	HelpVerifyExample = `  nomos verify
  nomos verify --all --strict
  nomos verify --fast-file substrate/cli/root.go
  nomos verify --impact`

	HelpVerifyProofExample = `  nomos verify proof
  nomos verify proof . --json`

	HelpPhysicsLong = `Compute Software Physics mass and real-time collision vectors across active
transient task worktrees. Inspects touched files, AST symbol exports, and package
fan-in/fan-out to classify concurrent worktree proximity as GREEN, YELLOW, or RED.`
	HelpPhysicsExample = `  nomos physics
  nomos physics --json`

	HelpHandshakeExample = `  nomos handshake
  nomos handshake --json`

	HelpHookExample = `  nomos hook pre-tool < payload.json
  nomos hook post-tool --hermetic-dir /workspace < payload.json`

	HelpHookSessionStartLong = `Ingest a JSON SessionStart hook payload from stdin, ensure repository Git
defense hooks are active, and emit a deterministic JIT workspace orientation banner.`
	HelpHookSessionStartExample = `  echo '{"cwd":"."}' | nomos hook session-start`

	HelpHookPreToolLong = `Ingest a JSON PreToolUse payload from stdin and enforce Law 3 worktree boundary
isolation, unexpanded shell variable hygiene, Law 7 command safety, and the 3-strike
identical-failure circuit-breaker before mutating tools execute.`
	HelpHookPreToolExample = `  echo '{"tool_name":"Write","tool_input":{"file_path":"main.go"}}' | nomos hook pre-tool`

	HelpHookPostToolLong = `Ingest a JSON PostToolUse payload from stdin, remain 100% silent on non-edit
channels to preserve native OS error provenance, and run zero-footprint in-memory
syntax and budget radar checks on modified source files.`
	HelpHookPostToolExample = `  echo '{"tool_name":"Edit","tool_input":{"file_path":"worktrees/repo-SUB-1/app.go"}}' | nomos hook post-tool`

	HelpHookStopLong = `Ingest a JSON Stop or AfterAgent turn-termination payload from stdin, honor
stop_hook_active re-entry safety, block unverified active EDIT worktree exits until
nomos verify executes, and block container-sandbox exits with syntax or verifier-path faults.`
	HelpHookStopExample = `  echo '{"hook_event_name":"Stop","cwd":"worktrees/repo-SUB-1"}' | nomos hook stop
  echo '{"hook_event_name":"AfterAgent","cwd":"/app"}' | nomos hook stop --container-sandbox`

	HelpHookInstallLong = `Additively install or merge Nomos lifecycle hooks (SessionStart, PreToolUse,
PostToolUse, and Stop/AfterAgent) across Claude Code, Gemini CLI, OpenCode, Cursor,
and Antigravity into project-scoped or user-scoped manifests without clobbering existing hooks.`
	HelpHookInstallExample = `  nomos hook install --claude --user
  nomos hook install --cursor --antigravity --dir ./my-service
  nomos hook install --gemini --opencode --dir ./my-service`

	HelpDirectiveExample = `  nomos directive
  nomos directive plan_03_nomos_controlled_technical_english.md
  nomos directive --lexicon --domain steering
  nomos directive --term sandbox`

	HelpCapabilitiesExample = `  nomos capabilities
  nomos capabilities --type gate
  nomos capabilities plane_separation --json`

	HelpMemExample = `  nomos mem recall "worktree"
  nomos mem list --limit 10
  nomos mem save --kind reflection --topic cli "Keep leaf commands self-documenting"
  nomos mem dismiss 42`

	HelpMemSaveLong = `Persist an advisory reflection, architectural axiom, or decision record into
Nomos Mneme SQLite storage partitioned to the active repository scope.`
	HelpMemSaveExample = `  nomos mem save "Prefer modular helper files under 300 LOC"
  nomos mem save --kind decision --topic verify --project nomos "Audit CLI help in Cell 6.2"`

	HelpMemRecallLong = `Query Nomos Mneme episodic and statutory memory records using hybrid lexical,
subword feature-hash, and optional local embedding similarity scoped to the active repository (or --cross-repo).`
	HelpMemRecallExample = `  nomos mem recall "plane separation"
  nomos mem recall "worktree" --cross-repo --limit 10 --json`

	HelpMemListLong = `Inspect stored Nomos Mneme reflections, axioms, decisions, learned pitfalls,
and verified patterns filtered by repository, task key, memory kind, or topic.`
	HelpMemListExample = `  nomos mem list
  nomos mem list --kind pattern --limit 10
  nomos mem list --all --json`

	HelpMemDismissLong = `Transition one or more Nomos Mneme memory records to tombstoned status once a
recurring pitfall has been promoted into nomos.yaml or a record is obsolete.`
	HelpMemDismissExample = `  nomos mem dismiss 42
  nomos mem dismiss 42 43 --json`

	HelpSessionExample = `  nomos session list
  nomos session show
  nomos session name "CLI Surface Hardening"`

	HelpSessionNameExample = `  nomos session name "Wave A Hardening"
  nomos session name --id 12579ec0 "Audit Probes" --json`

	HelpSessionListLong = `Discover and list active or historical Tier 1 coding agent conversation sessions
across supported IDE and CLI providers with status and last-active timestamps.`
	HelpSessionListExample = `  nomos session list
  nomos session list --json`

	HelpSessionVisualLong = `Synthesize a deterministic Mermaid topology graph linking active Tier 1 agent
sessions to workspace branches, tasks, and mission states.`
	HelpSessionVisualExample = `  nomos session visual
  nomos session visual --json`

	HelpSessionShowLong = `Inspect turn count, tool invocations, causal trajectory progress, and Sentinel
integrity scoring for a specific or active Tier 1 agent conversation session.`
	HelpSessionShowExample = `  nomos session show
  nomos session show 12579ec0 --json`

	HelpSessionExportLong = `Synthesize RLVR, DPO, and SFT dataset records by joining Tier 1 session trajectories,
Sentinel process-reward scorecards, Git Notes release receipts, and Mneme pitfalls/patterns.`
	HelpSessionExportExample = `  nomos session export --limit 25
  nomos session export --limit 10 --json`

	HelpIssueExample = `  nomos issue "Verify gate timeout"
  nomos issue --dry-run "CLI flag mismatch"
  nomos issue "AST parse anomaly" --output .tmp/report.md --body "Observed on monorepo"`

	HelpVersionLong = `Display the compiled Nomos Open-Core substrate version, target OS/architecture,
and Go toolchain provenance.`
	HelpVersionExample = `  nomos version
  nomos version --json`

	HelpUpdateLong = `Inspect release availability, enforce package-manager ownership guards (Homebrew, AUR, Nix),
verify checksums.txt SHA-256 digests in pure Go, atomically replace standalone binaries,
and optionally synchronize downstream workspace protocol blocks and ambient hooks.`
	HelpUpdateExample = `  nomos update
  nomos update --check --json
  nomos update --version v4.14.0
  nomos update --sync-workspace`
)
