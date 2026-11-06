---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-13"
last_updated: "2026-11-05"
plane: intent
ssot_repository: nomos
status: active
title: "Nomos Software Physics Engine - Kinematics, Continuous Collision Detection & Phase Ballistics Specification"
trigger: always_on
type: spec
version: "4.0.0"
---

# Nomos Software Physics Engine: Kinematics, Continuous Collision Detection & Phase Ballistics Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Formulate computational software physics across AST Mass, Shannon Entropy Viscosity, Continuous Collision Detection, and 2PC Re-entry. Ground all telemetry in graph theory and control systems.

---

## 1. Problem Statement & First-Principles Grounding

### 1.1 The Category Error: Code Is Machinery, Not Paint
*The Physics of Software: Why Code is Not Paint* grounds software engineering in computational physics rather than subjective canvas curation.

When autonomous coding agents operate under generative friction approaching zero, the cognitive cost of unverified code approaches infinity:

$$\lim_{\text{Friction}(\text{Generation}) \to 0} \text{CognitiveDebt}(\text{Verification}) = \infty$$

Without continuous physical constraints, agents drift into un-indexed loops, semantic signature collisions, and state-space combinatorial explosions. Post-hoc merge verification arrives too late - detecting catastrophic defects only after dissipating massive cognitive energy.

### 1.2 Separation of Substrate Physics from Ergonomic Telemetry
To maintain uncompromising realism and avoid metaphor degradation:
1. **The Software Physics Engine (Open-Core Substrate):** Pure Go telemetry structs calculate AST graph metrics, Shannon entropy, call-graph reachability, and capability leases.
2. **The Air Traffic Control Interface (HMI Projection):** `nomos physics` visualizes AST mass, worktree hulls, and collision threat levels for human architects. The visualization never replaces the math; it merely renders it at human-perceptual speed.

---

## 2. The 7 Laws of Software Physics

| Law # | Law Name | Physical Counterpart | Substrate Mechanical Invariant |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Aerodynamic Airframe vs. Pilot | Substrate Go core enforces flight laws over bounded Tier 1 agents. |
| **2** | **The Non-Verbal Proof Law** | Conservation of Mass & Momentum | AST and git invariants calculate mass, entropy, and collision vectors directly. |
| **3** | **The Hermetic Worktree Law** | Isolated Flight Corridors | Mutations occur strictly inside transient isolated worktrees; trunk `main` is physically immutable. |
| **4** | **The Single Source of Truth Law** | Unified Spacetime Metric | All physics telemetry binds strictly to compiled Go structs in `substrate/cli/physics.go`. |
| **5** | **The Zero-Hardcoded-Path Law** | Relativistic Reference Frames | Symbol coordinates and paths resolve dynamically relative to transient worktree roots. |
| **6** | **The Canonical Singularity Law** | Conservation of Energy | One canonical tensor governs mass, drag, and collision without shadow state. |
| **7** | **The Fail-Closed Pre-Flight Law** | Atmospheric Re-entry Envelope | DoD gate defects trigger immediate wave-offs (`git checkout -- <file>`). |

---

## 3. Mathematical Formulations

```text
                                  SOFTWARE PHYSICS TENSOR
                                             │
         ┌───────────────────┬───────────────┴───────────────┬───────────────────┐
         ▼                   ▼                               ▼                   ▼
    1. FILE MASS     2. VISCOSITY & DRAG             3. CONTINUOUS         4. 2PC RE-ENTRY
     & INERTIA        (Shannon Entropy)                COLLISION              ENVELOPE
   (McCabe/AST)         (Information)                 (CCD Graphs)          (42 DoD Gates)
```

### 3.1 Kinematics, File Mass & Inertia
Source files and packages acquire **Inertia** from AST cardinality, decision branching, and topological coupling rather than disk bytes.

#### Mass Metric:
For any source file $f \in \mathcal{F}$, the engine defines physical mass $M(f)$ as:

$$M(f) = \alpha \cdot \text{LOC}(f) + \beta \cdot v(G)_f + \gamma \cdot |\text{AST}_f| + \delta \cdot \left( \text{FanIn}(f) + \text{FanOut}(f) \right)$$

Where:
- $\text{LOC}(f) \in \mathbb{N}_0$: Physical non-blank lines of code.
- $v(G)_f \in \mathbb{N}_1$: Total McCabe Cyclomatic Complexity of all functions in $f$.
- $|\text{AST}_f| \in \mathbb{N}_0$: Total node cardinality in the Tree-sitter abstract syntax tree.
- $\text{FanIn}(f), \text{FanOut}(f) \in \mathbb{N}_0$: Number of external callers and external module dependencies.
- Standard default weighting coefficients: $\alpha = 0.1, \beta = 2.0, \gamma = 0.05, \delta = 1.5$.

#### Mutation Momentum & Kinetic Energy:
Let $v_{\text{edit}} = \frac{\Delta \text{AST}}{\Delta t}$ represent the editing velocity (AST mutations per second).
- **Momentum:** $p(f) = M(f) \cdot v_{\text{edit}}$
- **Kinetic Energy:** $E_k(f) = \frac{1}{2} M(f) \cdot v_{\text{edit}}^2$

High-mass files moving at high velocity have immense cognitive inertia; attempting to mutate them without braking controls introduces severe risk of catastrophic structural failure.

---

### 3.2 Cognitive Drag, Viscosity & Shannon Information Entropy
When source code exhibits high disorder, scattered naming, or structural redundancy, it creates cognitive drag that degrades both human comprehension and LLM attention allocation.

#### Shannon Token Entropy $H(X)$:
For a set of lexical tokens $T = \{t_1, t_2, \dots, t_n\}$ in file $f$:

$$H(f) = -\sum_{i=1}^n P(t_i) \log_2 P(t_i)$$

Where $P(t_i) = \frac{\text{count}(t_i)}{\sum_j \text{count}(t_j)}$ is the empirical probability of token $t_i$.

#### Structural Viscosity $\mu(f)$:
Code viscosity combines information entropy with sliding-window structural duplication:

$$\mu(f) = \frac{H(f)}{H_{\text{max}}} \cdot \left( 1 + \text{DuplicationRatio}(f) \right)$$

Rolling Rabin-Karp hashes over sliding token windows of size $w = 10$ compute $\text{DuplicationRatio}(f) \in [0.0, 1.0]$.

#### Aerodynamic Drag Force $F_{\text{drag}}$:
The drag force opposing clean cognitive traversal through the codebase is:

$$F_{\text{drag}}(f) = \frac{1}{2} \cdot C_d \cdot \mu(f) \cdot v_{\text{edit}}^2$$

Files with $F_{\text{drag}} > \tau_{\text{drag}}$ trigger immediate refactoring advisories prior to task release.

---

### 3.3 Continuous Collision Detection (CCD) in Multi-Agent Topologies
Standard version control (Git) relies on **discrete, post-hoc line collision detection** at merge time. If two agents modify distinct files, Git reports zero conflicts even if Agent A alters a public API that Agent B depends on.

In game physics, **Continuous Collision Detection (CCD)** eliminates the "tunneling problem" by sweeping bounding volumes through time. In software physics, Nomos executes Continuous Collision Detection across concurrent agent worktrees.

#### The Worktree Mutation Hull:
For any active transient worktree $W_k$ executing task $T_k$, the engine maintains an active **Mutation Hull** $\mathcal{H}(W_k)$:

$$\mathcal{H}(W_k) = \left\langle \text{ExportsMutated}(W_k), \text{ImportsReferenced}(W_k), \text{FilesTouched}(W_k) \right\rangle$$

#### The Real-Time Collision Tensor $C(W_i, W_j)$:
Between any two concurrent worktrees $W_i$ and $W_j$:

$$C(W_i, W_j) = \left( \text{ExportsMutated}(W_i) \cap \text{ImportsReferenced}(W_j) \right) \cup \left( \text{ExportsMutated}(W_j) \cap \text{ImportsReferenced}(W_i) \right)$$

#### Collision Threat Levels:
```text
┌──────────────────────┬───────────────────────────────────────────┬───────────────────────────────────────┐
│ Threat Level         │ Mathematical Condition                    │ Substrate Action                      │
├──────────────────────┼───────────────────────────────────────────┼───────────────────────────────────────┤
│ GREEN (Clear)        │ C(W_i, W_j) = ∅ ∧ FilesTouched ∩ Files = ∅│ Unconstrained parallel flight         │
│ YELLOW (Advisory)    │ FilesTouched(W_i) ∩ FilesTouched(W_j) ≠ ∅ │ Proximity warning to both agents      │
│ RED (Collision Imminent) │ C(W_i, W_j) ≠ ∅                           │ Immediate TCAS alert & lock resolution│
└──────────────────────┴───────────────────────────────────────────┴───────────────────────────────────────┘
```

When CCD detects a RED collision during active editing, the substrate alerts the orchestrator before either agent commits.

---

### 3.4 The 60Hz Telemetry Loop & Nyquist-Shannon Sampling
Under the Nyquist-Shannon sampling theorem, feedback loops must sample above twice the maximum disturbance frequency to prevent aliasing:

$$f_{\text{feedback}} > 2 \cdot f_{\text{mutation}}$$

When an agent streams AST mutations at 2Hz, the substrate evaluates AST, type, and boundary invariants at sub-second intervals. Sampling at $f \ge 4\text{Hz}$ (targeting 60Hz telemetry frames) maintains deterministic feedback.

This high-frequency loop ensures the agent stays strictly within its **Controllable Basin of Attraction**, preventing compounding hallucinations.

---

### 3.5 Two-Phase Commit (2PC) Atmospheric Re-entry
Trunk branch `main` represents the pristine surface of production reality. The engine models transient worktree merges as an **atmospheric re-entry maneuver**:

1. **Pre-Flight Entry Vector:** Phase transitions from `EDIT` to `REVIEW`. CapBAC leases freeze further file mutations.
2. **The DoD Thermal Shield:** `nomos verify` runs all Definition of Done quality gates against the worktree AST.
3. **Aerodynamic Wave-Off (Revert Invariant):** Any gate defect ($exit \ne 0$) aborts the landing immediately. The worktree executes:
   ```bash
   git checkout -- <file>
   ```
   No scorched earth or damaged state touches `main`.
4. **Touchdown & Release Receipt:** Passing all DoD gates triggers atomic 2PC merge to `main` and records a receipt in `refs/notes/nomos/releases`.

---

## 4. Cross-Plane Transduction Matrix & CLI Surface (`nomos physics`)

The Open-Core Software Physics Engine exposes real-time worktree AST mass, inertia, and continuous collision detection through `nomos physics [--json]` (registered under the `quality` CLI group).

| Intent Physical Vector | Substrate Engine Package | CLI & Telemetry Projection |
|---|---|---|
| **AST Mass & Inertia (`#FileMass`)** | `substrate/cli/physics.go`, `substrate/cli/cmd_physics.go` | `nomos physics [--json]` worktree mass & file breakdown |
| **Worktree Mutation Hull (`#WorktreeHull`)** | `substrate/cli/physics.go`, `substrate/cli/cmd_physics.go` | Active `worktrees/*` AST export/import & touched-file hulls |
| **Continuous Collision (`#CollisionVector`)** | `substrate/cli/physics.go` (`DetectCollision`) | `GREEN` / `YELLOW` / `RED` pairwise collision alerts in `nomos physics` |
| **High-Frequency Phase Loop** | `substrate/src/nomos/modules/telemetry` | Structured JSON telemetry snapshots |
| **2PC Re-entry Touchdown** | `substrate/src/nomos/modules/task/git_notes.go` | Cryptographic release receipt in Git Notes (`refs/notes/nomos/releases`) |

---

## 5. Table-Driven Verification Vectors

| Vector ID | Physical Invariant | Input Workspace State | Expected Verdict |
| :--- | :--- | :--- | :--- |
| `TC-PHY-01` | AST Mass & Complexity | File LOC `> 400` or function cyclomatic complexity `> 15` | `FAIL` (Mass budget exceeded) |
| `TC-PHY-02` | Continuous Collision (CCD) | Disjoint file sets across parallel worktrees ($C(W_i, W_j) = \emptyset$) | `PASS` (`GREEN` clear parallel flight) |
| `TC-PHY-03` | Continuous Collision (CCD) | Overlapping symbol/file mutation ($C(W_i, W_j) \ne \emptyset$) | `WARN/BLOCK` (`RED` collision alert) |
| `TC-PHY-04` | 2PC Atmospheric Re-entry | Any of the 42 DoD gates exits non-zero during `nomos task sync` | `FAIL` (Wave-off; trunk `main` untouched) |

---

## 6. Definition of Done & Verification Boundary

1. Compiled Go physics telemetry structs (`WorktreeHull`, `CollisionVector`, `FileMass`) enforced in `substrate/cli/physics.go`.
2. All 42 DoD quality gates pass cleanly via `nomos verify`.
3. Zero shims, zero placeholders, and strict adherence to the 7 Laws of the Nomos Open-Core Protocol.
