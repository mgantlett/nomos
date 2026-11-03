---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-23"
last_updated: "2026-11-01"
plane: intent
status: active
title: "Nomos Mneme Episodic and Semantic Memory Subsystem Specification"
type: spec
version: "4.0.0"
tags:
    - INT-735
    - SUB-1213
    - INT-855
    - SUB-1524
ssot_repository: nomos
---

# Nomos Mneme Episodic and Semantic Memory Subsystem Specification

## 1. Executive Summary & Problem Statement

In autonomous multi-task engineering cycles, Tier 1 coding agents frequently repeat identical mistakes across sessions unless provided with persistent episodic memory. Conversely, monolithic context dumping rapidly consumes the agent's context window.

Under **Law 1 (The Dual-Core Inversion Law)** and **Law 4 (The Single Source of Truth Law)**, Nomos provides **Nomos Mneme**: a pure-Go, transactional episodic and vector memory store (`mneme.db`) that records post-task reverse-reflections, session axioms, and architectural decisions.

Nomos Mneme operates with **zero CGO dependencies**, using deterministic local vector synthesis (`GenerateDeterministicLocalVector`) and hybrid ranking (cosine similarity + lexical overlap + Ebbinghaus temporal decay), partitioned strictly per repository (`repo_id`).

---

## 2. Axiomatic Invariants & Laws Codified

| Law # | Law Name | Architectural Invariant | Machine Enforcement Mechanism |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Memory recall and persistence are governed by the substrate; Tier 1 agents query memories under indirect control. | Canonical CLI commands (`nomos mem`) |
| **2** | **The Non-Verbal Proof Law** | Memory embeddings and ranking algorithms produce deterministic mathematical scores. | Unit test vectors, cosine bounded in $[-1.0, 1.0]$ |
| **4** | **The Single Source of Truth Law** | Memory record schema binds directly to compiled `mem.Memory` (`substrate/src/nomos/modules/mem/memory.go`). | Relational DDL matching `mem.Memory` |
| **5** | **The Zero-Hardcoded-Path Law** | Database path dynamically resolves via `config.ResolveMnemeDBPath()`. | Dynamic configuration discovery |
| **6** | **The Canonical Singularity Law** | Exactly three core memory actions: `save`, `recall`, and `list`. Zero redundant aliases. | CLI surface validation (`cmd/nomos`) |
| **7** | **The Fail-Closed Pre-Flight Law** | Direct SQLite database queries via raw `sqlite3` CLI are prohibited (Database Encapsulation Invariant). | Static gate and hook rules |

---

## 3. Relational Database Schema (`mneme.db`)

The SQLite persistence schema matches the canonical Go struct `mem.Memory` defined in `substrate/src/nomos/modules/mem/memory.go`:

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
- `repo_id`: The canonical repository identifier (e.g. `nomos`, `acme-backend`).
- `kind`: Enumeration partitioned by write provenance:
  - **Advisory Kinds** (eligible for CLI `nomos mem save`): `reflection`, `axiom`, `decision`, `fact`, `preference`, `architecture`.
  - **Statutory Kinds** (minted strictly by internal 2PC `nomos task sync` after 42/42 DoD verification): `pitfall` (negative-path resolved gate trap), `pattern` (positive-path verified structural blueprint), `release`, `task`, `commit`, `review`, `legacy_spec`.
- `topic`: Categorization tag (e.g. `Architecture`, `Verification`, `pattern:substrate/src/nomos/modules/mem`).
- `content`: Markdown text of the reflection, pitfall fix, or verified structural blueprint.
- `task_id`: Linked task key (e.g. `INT-734`, `SUB-1212`).
- `embedding`: 64-dimensional float32 vector encoded as little-endian binary blob (256 bytes).
- `status`: Lifecycle state (`active`, `superseded`, `tombstoned`).

---

## 4. Pure-Go Vector Synthesis & Hybrid Ranking Engine

### 4.1 Deterministic Local Vector Synthesis
To eliminate external inference dependencies and CGO libraries, Nomos implements **Deterministic Local Content Hashing**:
1. Tokenizes content into lowercase alphanumeric words.
2. Computes SHA-256 digests per token and projects byte values into a 64-dimensional float space.
3. Normalizes the resulting vector to unit Euclidean magnitude ($|v| = 1$).

### 4.2 Hybrid Ranking Formula
When searching memories via `nomos mem recall <query>`:
$$\text{Score} = (0.75 \times \text{CosineSimilarity} + 0.25 \times \text{LexicalOverlap}) \times \text{TemporalDecay} \times \text{Reinforcement}$$

- **Cosine Similarity**: $\frac{A \cdot B}{\|A\| \|B\|}$ evaluating semantic alignment.
- **Lexical Overlap**: Word token Jaccard-style intersection over query length.
- **Ebbinghaus Temporal Decay**: $\exp(-\lambda \times \text{age}_{\text{days}})$, where $\lambda$ is tuned by memory kind (`axiom`: $0.005$, `pattern`: $0.006$, `pitfall`: $0.008$, `reflection`: $0.02$, `release`/`task`/`commit`: $0.05$).
- **Access Reinforcement**: $1.0 + 0.05 \times \min(\text{access\_count}, 10)$, reinforcing frequently accessed wisdom and tracking Tier A $\to$ Tier B graduation candidates (`access_count >= 3`).

---

## 5. Canonical Command Surface (`nomos mem`)

### 5.1 `nomos mem save <content>`
- Saves an advisory knowledge item into Nomos Mneme.
- **Statutory Provenance Guard (Law 1 & Law 2):** Manual CLI invocations passing statutory kinds (`pitfall`, `pattern`, `release`, `task`, `commit`, `review`) fail closed (`ERR_VALIDATION_FAILED`) so Tier 1 agents cannot spoof verified 2PC release artifacts.
- Flags:
  - `-p, --project <repo>`: Scope to repository (defaults to active repo).
  - `--topic <topic>`: Topic category (default: `General`).
  - `--kind <kind>`: Advisory kind of memory (`reflection`, `axiom`, `decision`; default: `reflection`).

### 5.2 `nomos mem recall <query>`
- Hybrid semantic and lexical search.
- Scoped to active repository by default to prevent cross-project pollution.
- Flags:
  - `-A, --cross-repo`: Cross-repository search across all indexed memories.
  - `-p, --project <repo>`: Explicit repository target.
  - `-n, --limit <int>`: Result limit (default: 5).
  - `--json`: Structured JSON output.

### 5.3 `nomos mem list`
- Inspects recorded memories with table formatting.
- Flags:
  - `-A, --all`: Show memories across all repositories.
  - `-p, --project <repo>`: Filter by repository.
  - `-n, --limit <int>`: Max results (default: 20).
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
| `TC-MEM-08` | Tier B graduation surfacing | Statutory memory reaches `access_count >= 3` | Surfaced in `graduation_candidates` for promotion into `nomos.yaml`. |
