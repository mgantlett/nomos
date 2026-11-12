---
authors:
  - Mark Gantlett
  - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-11"
lineage:
  - INT-735
  - INT-855
  - INT-899
  - INT-926
  - INT-927
  - INT-938
  - SUB-1213
  - SUB-1524
  - SUB-1680
  - SUB-1806
  - INT-1944
  - SUB-2157
  - INT-1956
  - SUB-2203
  - INT-1962
  - SUB-2211
  - INT-1984
  - SUB-2279
plane: intent
ssot_repository: nomos
status: active
title: "Nomos Mneme Receipt-Gated Episodic Memory Subsystem Specification"
type: spec
version: "4.15.0"
---

# Nomos Mneme Receipt-Gated Episodic Memory Subsystem Specification

---

## 1. Executive Summary & Problem Statement

In autonomous multi-task engineering cycles, Tier 1 coding agents repeat identical mistakes across sessions without persistent episodic memory. Conversely, monolithic context dumping rapidly consumes the agent's context window.

Under **Law 1** and **Law 4**, **Nomos Mneme** (`mneme.db`) provides a pure-Go receipt-gated transactional episodic store for reflections, axioms, pitfalls, and structural patterns.

Nomos Mneme pairs AST-scoped pitfall injection with zero-CGO subword hashing (`GenerateDeterministicLocalVector`), lexical overlap, Ebbinghaus decay, and optional `LOCAL_EMBEDDING_URL` vectors per `repo_id`.

---

## 2. Axiomatic Invariants & Governing Laws

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Substrate governs memory storage; Tier 1 agents query under indirect control. | Canonical CLI commands (`nomos mem`) |
| **2** | **The Non-Verbal Proof Law** | Memory embeddings and ranking algorithms produce deterministic mathematical scores. | Unit test vectors, cosine bounded in $[-1.0, 1.0]$ |
| **4** | **The Single Source of Truth Law** | Memory record schema binds directly to compiled `mem.Memory` (`substrate/src/nomos/modules/mem/memory.go`). | Relational DDL matching `mem.Memory` |
| **5** | **The Zero-Hardcoded-Path Law** | Database path dynamically resolves via `config.ResolveMnemeDBPath()`. | Dynamic configuration discovery |
| **6** | **The Canonical Singularity Law** | Four core actions (`save`, `recall`, `list`, `dismiss`) with zero aliases. | `Cell 6.2 (zero_alias)` CLI surface validation |
| **7** | **The Fail-Closed Pre-Flight Law** | Prohibits raw `sqlite3` CLI queries under the Database Encapsulation Invariant. | `Cell 1.1 (boundary)` and hook rules |

---

## 3. Compiled Go SSoT & Relational Database Schema (`mneme.db`)

Package `substrate/src/nomos/modules/mem/memory.go` defines the compiled Go Single Source of Truth for episodic memory records and statutory provenance validation:

```go
package mem

import "time"

// MemoryKind categorizes cognitive memory entries.
type MemoryKind string

// MemoryStatus defines the lifecycle state of an episodic memory.
type MemoryStatus string

const (
	StatusActive     MemoryStatus = "active"
	StatusSuperseded MemoryStatus = "superseded"
	StatusTombstoned MemoryStatus = "tombstoned"
)

// Memory represents a single knowledge item indexed in Nomos Mneme (#Memory).
type Memory struct {
	ID           int          `json:"id"`
	RepoID       string       `json:"repo_id,omitempty"`
	Kind         MemoryKind   `json:"kind"`
	Topic        string       `json:"topic"`
	Content      string       `json:"content"`
	TaskID       string       `json:"task_id"`
	Embedding    []float32    `json:"embedding,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
	Status       MemoryStatus `json:"status"`
	SupersededBy int          `json:"superseded_by,omitempty"`
	AccessCount  int          `json:"access_count"`
}

// MemoryFilter specifies filtering criteria for listing recorded memories (#MemoryFilter).
type MemoryFilter struct {
	TaskID           string
	Kind             MemoryKind
	Topic            string
	Status           MemoryStatus
	RepoID           string
	IncludeCrossRepo bool
	Limit            int
}

// SearchOptions configures hybrid semantic and lexical memory search (#SearchOptions).
type SearchOptions struct {
	Limit            int
	Kind             MemoryKind
	Topic            string
	RepoID           string
	IncludeCrossRepo bool
	IncludeInactive  bool
	ReadOnly         bool
}

// SearchResult pairs a matched Memory record with its hybrid relevance score (#SearchResult).
type SearchResult struct {
	Memory  *Memory `json:"memory"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

// GateTrapEntry records a buffered quality gate failure during a task session (#GateTrapEntry).
type GateTrapEntry struct {
	GateName  string `json:"gate_name"`
	Message   string `json:"message"`
	Scope     string `json:"scope"`
	Timestamp string `json:"timestamp"`
}

// PitfallHygieneReport summarizes automated pitfall scope pruning and cross-scope promotion (#PitfallHygieneReport).
type PitfallHygieneReport struct {
	DeadScopesTombstoned    int `json:"dead_scopes_tombstoned"`
	ExpiredTestsTombstoned  int `json:"expired_tests_tombstoned"`
	CrossScopeHabitsUpdated int `json:"cross_scope_habits_updated"`
}

// MaintainReport summarizes deduplication, retention, VACUUM, and WAL checkpoint actions (#MaintainReport).
type MaintainReport struct {
	DuplicatesSuperseded int  `json:"duplicates_superseded"`
	InactivePurged       int  `json:"inactive_purged"`
	Vacuumed             bool `json:"vacuumed"`
	WALCheckpointed      bool `json:"wal_checkpointed"`
}

// HotScopeReflex describes an active directory-scoped post-tool AST reflex (#HotScopeReflex).
type HotScopeReflex struct {
	Scope           string   `json:"scope"`
	RecurrenceDepth int      `json:"recurrence_depth"`
	CleanStreak     int      `json:"clean_streak"`
	ActiveGates     []string `json:"active_gates"`
}

// ValidateAdvisoryWriteKind enforces statutory write provenance for manual CLI saves (@intent: #Memory).
func ValidateAdvisoryWriteKind(kind MemoryKind) error

// ResolveHotScopeReflex inspects active directory pitfalls for post-tool AST sensitization (@intent: #HotScopeReflex).
func ResolveHotScopeReflex(repoRoot, relDir string) *HotScopeReflex
```

The SQLite persistence schema matches `mem.Memory`:

```sql
CREATE TABLE IF NOT EXISTS memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    repo_id TEXT DEFAULT '',
    kind TEXT NOT NULL,
    topic TEXT NOT NULL,
    content TEXT NOT NULL,
    task_id TEXT NOT NULL,
    embedding BLOB,
    created_at DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    superseded_by INTEGER DEFAULT 0,
    access_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_memories_topic ON memories(topic);
CREATE INDEX IF NOT EXISTS idx_memories_status ON memories(status);
CREATE INDEX IF NOT EXISTS idx_memories_repo ON memories(repo_id);
CREATE INDEX IF NOT EXISTS idx_memories_kind ON memories(kind);
CREATE INDEX IF NOT EXISTS idx_memories_repo_kind_status ON memories(repo_id, kind, status);
```

### 3.1 Field Specifications
- `repo_id`: The canonical repository identifier (such as `nomos` or `acme-backend`).
- `kind`: Enumeration partitioned by write provenance:
  - **Advisory Kinds** (eligible for CLI `nomos mem save`): `reflection`, `axiom`, `decision`, `fact`, `preference`, `architecture`.
  - **Statutory Kinds** (minted strictly by 2PC `nomos task sync` after 42/42 verification): `pitfall`, `pattern`, `release`, `task`, `commit`, `review`, and `legacy_spec`.
- `topic`: Categorization label (such as `Architecture`, `Verification`, or `pattern:substrate/src/nomos/modules/mem`).
- `content`: Markdown text of the reflection, pitfall fix, or verified structural blueprint.
- `task_id`: Linked task key (`INT-734`, `SUB-1212`).
- `embedding`: Little-endian `float32` binary blob (`256` bytes for 64-dim offline subword vectors, or dense neural vectors from `LOCAL_EMBEDDING_URL`).
- `status`: Lifecycle state (`active`, `superseded`, `tombstoned`).

---

## 4. Pure-Go Subword Feature Hashing & Hybrid Ranking Engine

### 4.1 Subword Feature Hashing & Optional Local Embeddings
To eliminate external inference dependencies and CGO libraries, Nomos implements **Subword Feature Hashing** with optional local neural fallback:
1. Splits `camelCase`, `PascalCase`, `snake_case`, and path tokens into lowercase words, strips high-frequency English stop-words, and extracts character 4-grams (`0.35` weight) alongside normalized tokens (`1.0` weight).
2. Projects each weighted feature across 64 independent zero-mean coordinates via domain-separated SHA-256 digests (`0:` for `[0..31]`, `1:` for `[32..63]`), normalizing to unit magnitude ($|v| = 1$).
3. Queries `LOCAL_EMBEDDING_URL` (`127.0.0.1:8081/v1/embeddings`, `500ms` timeout) when active, and synthesizes 64-dimensional subword vectors in `CombinedScore` whenever stored and query dimensions differ.

### 4.2 Hybrid Ranking Formula
When searching memories via `nomos mem recall <query>`:
$$\text{Score} = (0.75 \times \text{CosineSimilarity} + 0.25 \times \text{LexicalOverlap}) \times \text{TemporalDecay} \times \text{Reinforcement} \times \text{LocalityBoost}$$

- **Cosine Similarity**: $\frac{A \cdot B}{\|A\| \|B\|}$ evaluating subword feature-hash or local neural alignment.
- **Lexical Overlap**: Identifier-split, stop-word-filtered token intersection over query length.
- **Ebbinghaus Temporal Decay**: $\exp(-\lambda \times \text{age}_{\text{days}})$, with kind-specific $\lambda$ (`axiom`: $0.005$, `pattern`: $0.006$, `pitfall`: $0.008$, `reflection`: $0.02$, `release`/`task`/`commit`: $0.05$).
- **Repository Locality Boost & Global Axiom Slot Cap**: Scoped searches (`opts.RepoID != ""` and `!opts.IncludeCrossRepo`) apply a $1.25\times$ multiplier to exact `repo_id` matches. Untyped repository queries (`opts.Kind == ""`) cap unscoped `global` `axiom` slots at $\max(1, \lfloor \text{limit} / 3 \rfloor)$ and backfill from overflow only when repository matches are sparse.
- **Access Reinforcement & Per-Task Deduplication**: Applies $1.0 + 0.05 \times \min(\text{access\_count}, 10)$ during active worktree tasks at most once per `(task_id, memory_id)`. Passive `IDLE` orientation (`nomos handshake`, `nomos status`, `nomos task status`) and `nomos mem recall` run read-only (`SearchOptions.ReadOnly = true`) without inflating `access_count`.
- **Recurrence-Aware Graduation & Clean-Streak Cooling (`RecallGraduationCandidates`)**: Built-in structural AST gates (`isBuiltInStructuralASTTopic`) bypass `nomos.yaml` graduation banners in Open-Core handshakes and route into `HotScopeReflex`. Policy-graduatable statutory pitfalls surface when topic recurrence $\ge 2$ or task-deduplicated `access_count >= 3`. Active head pitfalls cool out of `graduation_candidates` once `access_count >= 5` consecutive clean task recalls complete without re-tripping.
- **Closed-Loop Hot-Scope AST Reflexes (`ResolveHotScopeReflex`)**: When built-in AST pitfalls in `relDir` reach recurrence $\ge 3$ with head `access_count < 5`, `ResolveHotScopeReflex` activates sensitized `EvaluateFastFileRadar` checks. Sensitized scopes tighten `V(G)` and `LOC` warnings from `80%` to `65%` and run inline `arity`, `unchecked_error_hygiene`, and `anti_shim` micro-probes. Reaching head `access_count >= 5` consecutive clean recalls cools the scope back to baseline `80%`.
- **Symbol-Delta Pitfall Distillation (`DistillTaskLearnings`)**: When a buffered gate failure cites a repository file, 2PC sync parses verified post-fix AST symbols. Distillation prepends `Refactored <file> [<symbols>]` ahead of walkthrough prose.
- **Unscoped One-Off Pitfall Cooling (`RecallRepoPitfalls`)**: During unscoped handshakes (`targetScope == "workspace"`), single-occurrence pitfalls (`recurrence == 1`) older than 14 days cool out of `LearnedPitfalls`. Pitfalls remain active whenever `modifiedFiles` matches their directory scope or topic recurrence reaches $\ge 2$.
- **JIT Post-Edit Pitfall Recall & Trunk Pacing (`nomos hook post-tool`)**: First worktree mutation in `relDir` recalls matching directory pitfalls once via `RecallRepoPitfallsForTask` and records `relDir` in session trace. Trunk sessions (`wtRoot == ""`, `SessionID != ""`) track consecutive exploratory reads per session and emit pacing advisories at configured thresholds.

---

## 5. Canonical Command Surface (`nomos mem`)

### 5.1 `nomos mem save <content>`
- Saves an advisory knowledge item into Nomos Mneme.
- **Statutory Provenance Guard (Law 1 & Law 2):** Rejects manual CLI statutory kinds (`pitfall`, `pattern`, `release`, `task`, `commit`, `review`) with `ERR_VALIDATION_FAILED`.
- Flags:
  - `-p, --project <repo>`: Scope to repository (defaults to active repo).
  - `--topic <topic>`: Topic category (default: `General`).
  - `--kind <kind>`: Advisory kind of memory (`reflection`, `axiom`, `decision`; default: `reflection`).

### 5.2 `nomos mem recall <query>`
- Hybrid lexical, subword feature-hash, and optional local embedding search.
- Scoped to active repository by default to prevent cross-project pollution.
- Flags:
  - `-A, --cross-repo`: Cross-repository search across all indexed memories.
  - `-p, --project <repo>`: Explicit repository target.
  - `-n, --limit <int>`: Result limit (default: 5).
  - `--json`: Structured JSON output.

### 5.3 `nomos mem list`
- Lists recorded memories in table format, defaulting to `active` status and omitting `superseded` or `tombstoned` records unless `--status` overrides filtering.
- Flags:
  - `--status <status>`: Lifecycle filter (`active`, `superseded`, `tombstoned`, or `all`; default: `active`).
  - `-A, --all`: Show memories across all repositories.
  - `-p, --project <repo>`: Filter by repository.
  - `-n, --limit <int>`: Max results (default: 20).
  - `--json`: Structured JSON output.

### 5.4 `nomos mem dismiss <id> [id...]`
- Transitions active or superseded records to `tombstoned` (`Store.Dismiss`) after `nomos.yaml` codifies a `pitfall` or obsoletes an advisory item. Dismissal preserves `mneme.db` audit history under the Database Encapsulation Invariant without raw SQLite mutations.
- Flags:
  - `--json`: Structured JSON output.

---

## 6. Table-Driven Verification Vectors

| Vector ID | Evaluation Subject | Condition | Expected Outcome |
| :--- | :--- | :--- | :--- |
| `TC-MEM-01` | Save memory | Valid content | Saved to `mneme.db`; ID allocated; unit vector generated. |
| `TC-MEM-02` | Local repository scoping | Query without `-A` | Matches only memories matching current `repo_id` or `global`. |
| `TC-MEM-03` | Cross-repository search | Query with `-A` | Matches memories across all repositories. |
| `TC-MEM-04` | Deterministic embedding | Identical text | Yields identical 64-dimensional float vector with unit length. |
| `TC-MEM-05` | Empty content rejection | `nomos mem save ""` | Fails closed: memory content cannot be empty. |
| `TC-MEM-06` | Statutory provenance guard | `nomos mem save "fake" --kind pattern` or `--kind pitfall` | Fails closed (`ERR_VALIDATION_FAILED`): statutory kinds minted strictly by 2PC `nomos task sync`. |
| `TC-MEM-07` | Positive-path pattern distillation | Clean 42/42 verified `nomos task sync` | Distills `kind="pattern"` blueprint into `mneme.db` and recalls via `verified_patterns`. |
| `TC-MEM-08` | Recurrence-aware graduation surfacing | Policy pitfall recurs `>= 2` or `access_count >= 3` (`< 5`) | Surfaced in `graduation_candidates`; built-in AST gates route to `HotScopeReflex`. |
| `TC-MEM-09` | Memory dismissal | `nomos mem dismiss <id>` | Transitions record to `tombstoned`; excluded from active recall and `graduation_candidates`. |
| `TC-MEM-10` | Per-task reinforcement & read-only CLI recall | Repeated `RecallRepoPitfallsForTask` in same task + `SearchOptions{ReadOnly: true}` | Increments `access_count` `+1` per distinct task and `+0` on read-only recall. |
| `TC-MEM-11` | Symbol-delta pitfall fix distillation | Gate trap cites a repository source file resolved at 2PC sync | `Verified Fix` records concrete refactored file and verified symbols ahead of walkthrough prose. |
| `TC-MEM-12` | Unscoped one-off pitfall cooling | Single-occurrence pitfall (`recurrence == 1`) older than 14 days during unscoped handshake | Excluded from unscoped `LearnedPitfalls`; retained on scope match or `recurrence >= 2`. |
| `TC-MEM-13` | Active-default `nomos mem list` | `nomos mem dismiss <id>` followed by `nomos mem list` vs `nomos mem list --status tombstoned` | Excluded from default `nomos mem list` output and visible under `--status tombstoned` or `--status all`. |
| `TC-MEM-14` | Repo locality boost & axiom cap | Untyped `SearchWithOptions` (`RepoID: "nomos"`, `Limit: 5`) | Boosts `nomos` matches $1.25\times$ and caps `global` axioms to 1. |
| `TC-MEM-15` | Subword hashing & dimension fallback | `GenerateDeterministicLocalVector` + `CombinedScore` across `camelCase`, stop-words, and mixed dims | Produces independent halves (`v[:32] != v[32:]`), rejects stop-word noise, and scores mixed dims. |
| `TC-LRN-01` | Gate trap buffering | Failing DoD gate evaluated during active task (`GateTrapEntry`) | Buffers gate name, scope, and failure message for `LEARN` phase distillation. |
| `TC-LRN-02` | Trap-to-fix distillation | Buffered `GateTrapEntry` resolved before 2PC `nomos task sync` | Mints statutory `kind="pitfall"` pairing trap and verified fix. |
| `TC-LRN-03` | Dead-scope pitfall hygiene | `PitfallHygieneReport` audit when target directory no longer exists | Tombstones dead-scope pitfalls automatically. |
| `TC-LRN-04` | Cross-scope habit promotion | Identical pitfall recurring across $\ge 3$ distinct package scopes | Promotes pitfall scope to repository-wide orientation. |
| `TC-LRN-05` | Duplicate memory superseding | `MaintainReport` deduplication pass on identical topic and content | Marks older duplicate `superseded` and preserves newest record. |
| `TC-LRN-06` | `mneme.db` WAL checkpoint | Periodic maintenance pass over `mneme.db` (`MaintainReport`) | Checkpoints WAL and reclaims fragmented pages cleanly. |
| `TC-MNM-030` | JIT pitfall recall & trunk pacing | First `post-tool` edit in `relDir`; consecutive trunk reads | Recalls scoped pitfall once per `relDir`; paces trunk reads (`7/12`, `12/20`). |
| `TC-MEM-16` | Hot-scope AST reflex sensitization | Built-in AST pitfall reaches `recurrence >= 3` and `access_count < 5` | `ResolveHotScopeReflex` activates `65%` warnings plus `arity`, `unchecked_error_hygiene`, and `anti_shim` probes. |
| `TC-MEM-17` | Clean-streak homeostatic cooling | Head pitfall reaches `access_count >= 5` consecutive clean recalls | Cools `ResolveHotScopeReflex` to `nil` and suppresses `graduation_candidates` nagging. |
| `TC-MEM-18` | Built-in AST banner suppression | Built-in AST pitfall recurs `>= 2` times during handshake | Excluded from `nomos.yaml` graduation banners; policy pitfalls remain surfaced. |

---

## 7. Cross-Plane Traceability & DoD Verification

- **Intent Lineage**: `INT-735`, `INT-855`, `INT-899`, `INT-926`, `INT-927`, `INT-1944`, `INT-1956`, `INT-1962`, `INT-1984`.
- **Substrate SSoT**: `SUB-1213`, `SUB-1524`, `SUB-1680`, `SUB-2157`, `SUB-2203`, `SUB-2211`, `SUB-2279` (`substrate/src/nomos/modules/mem/memory.go`, `substrate/src/nomos/modules/mem/vector.go`, `substrate/src/nomos/modules/mem/search.go`, `substrate/src/nomos/modules/mem/learn.go`, `substrate/src/nomos/modules/mem/learn_reinforce.go`, `substrate/src/nomos/modules/mem/learn_scope.go`, `substrate/src/nomos/modules/mem/patterns.go`, `substrate/src/nomos/modules/mem/patterns_recall.go`, `substrate/src/nomos/modules/mem/maintain.go`, `substrate/src/nomos/modules/verify/fast_radar.go`, `substrate/cli/cmd_hook_post.go`, `substrate/cli/cmd_hook_pacing.go`, `substrate/cli/cmd_hook_trace.go`).
- **Canonical 7x3 Matrix Coordinates**: `Cell 1.1 (boundary)`, `Cell 4.1 (intent_traceability)`, `Cell 6.2 (zero_alias)`.
- **Deterministic Verification**: `nomos verify` executes unit tests across `substrate/src/nomos/modules/mem/` and validates Database Encapsulation and CLI zero-alias invariants with exit code `0`.
