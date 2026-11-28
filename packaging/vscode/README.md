# Nomos - AI Coding Guardrails & Codebase X-Ray

Zero-token, Apache-2.0 Go execution substrate between any coding agent (Claude Code, Cursor, Antigravity, Windsurf, OpenCode, Codex, Copilot) and Git.

## What It Does

- **Zero-Write Codebase X-Ray (`Nomos: Run Codebase X-Ray`):** Scans 11 languages in under 2 seconds without creating files or touching Git, computes your `AI Hangover Score (0-100)`, and surfaces ranked structural hotspots directly in the status bar and Problems panel.
- **Sub-15ms On-Save AST Radar:** Evaluates modified files on save via `nomos verify --fast-file` (when `nomos` is installed locally) to catch skipped tests, hollow stubs, swallowed errors, complexity spikes, and secret path leaks before commit.
- **42 Polyglot Quality Gates (`Nomos: Verify Workspace`):** Streams OASIS SARIF 2.1.0 findings from `nomos verify --sarif` into the Problems panel.
- **Stealth Day-0 Branch Guard (`Nomos: Initialize Branch Guard (Stealth --local)`):** Locks `main`, freezes existing legacy debt in `nomos.baseline.json`, and installs Git + agent hooks without adding tracked files to `git status`.
- **Native VS Code 1.148 Agent Integration:** Auto-registers the read-only `nomos mcp` stdio server (`mcpServerDefinitionProviders`) exposing `nomos://skill` and the Fullscreen MCP App (`ui://nomos/xray`), runs directly inside Automatic Ubuntu Dev Containers and Remote hosts (`extensionKind: ["workspace", "ui"]`), and wires `.github/hooks/nomos.json` via `Nomos: Initialize VS Code Agent Hooks`.
- **Shareable Markdown Receipt (`Nomos: Copy X-Ray Markdown Receipt`):** Copies a formatted `nomos xray . --markdown` scorecard for PRs and architecture reviews.

## Binary Provisioning & Air-Gap Safety

- Zero background network calls on activation.
- Resolution order: `nomos.binaryPath` setting -> system `PATH` -> cached extension storage -> explicit user-consented, SHA-256 verified download from [GitHub Releases](https://github.com/mgantlett/nomos/releases).
- Zero telemetry and zero phone-home.

## Links

- Documentation & Quickstart: https://sophialabs.ca/docs/introduction
- Open-Core Repository: https://github.com/mgantlett/nomos
- Zero-Install Web X-Ray: https://sophialabs.ca/xray
