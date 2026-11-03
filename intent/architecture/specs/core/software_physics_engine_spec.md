---
authors:
    - Mark Gantlett
    - Nomos Open-Core Substrate
classification: public
date: "2026-10-13"
last_updated: "2026-11-01"
plane: intent
status: active
title: Nomos Software Physics Engine - Kinematics, Continuous Collision Detection & Phase Ballistics Specification
trigger: always_on
type: spec
version: "4.0.0"
ssot_repository: nomos
---

# Nomos Software Physics Engine: Kinematics, Continuous Collision Detection & Phase Ballistics Specification

**Role:** Autonomous Open-Core Software Architect  
**Objective:** Formulate and codify the first-principles computational physics of software development (AST Mass, Shannon Entropy Viscosity, Multi-Worktree Continuous Collision Detection, and 2PC Atmospheric Re-entry). Ground all telemetry and ergonomic representations in mathematical computer science, graph theory, and control systems.

---

## 1. Problem Statement & First-Principles Grounding

### 1.1 The Category Error: Code Is Machinery, Not Paint
As established in the foundational Nomos Software Physics formulation (*The Physics of Software: Why Code is Not Paint*), software development is governed by the structural physics of computation rather than the subjective curation of paint on canvas.

When autonomous coding agents operate under generative friction approaching zero, the cognitive cost of unverified code approaches infinity:

$$\lim_{\text{Friction}(\text{Generation}) \to 0} \text{CognitiveDebt}(\text{Verification}) = \infty$$

Without continuous, physical constraints, agents drift into un-indexed loops, semantic signature collisions, and state-space combinatorial explosions. Post-hoc verification at merge time is fundamentally too late—it detects catastrophic failures only after massive cognitive energy has been dissipated.

### 1.2 Separation of Substrate Physics from Ergonomic Telemetry
To maintain uncompromising realism and avoid metaphor degradation:
1. **The Software Physics Engine (Open-Core Substrate):** Pure Go mathematical calculations and compiled Go telemetry structs evaluating AST graph metrics, Shannon information entropy, directed call-graph reachability, and capability leases.
2. **The Air Traffic Control Interface (HMI / Presentation Layer):** The ergonomic telemetry projection (`nomos physics`) that visualizes these underlying physics vectors (AST mass, worktree hulls, collision threat levels) for human architects. The visualization never replaces the math; it merely renders it at human-perceptual speed.

---

## 2. The 7 Laws of Software Physics

| Law # | Law Name | Physical Counterpart | Substrate Mechanical Invariant |
| :---: | :--- | :--- | :--- |
| **1** | **The Dual-Core Inversion Law** | Aerodynamic Airframe vs. Pilot | Substrate Go core enforces flight laws; Tier 1 agent is a bounded cognitive craft under indirect control. |
| **2** | **The Non-Verbal Proof Law** | Conservation of Mass & Momentum | Zero trust in textual claims; code mass, entropy, and collision vectors are calculated from AST and git invariants. |
| **3** | **The Hermetic Worktree Law** | Isolated Flight Corridors | Mutations occur strictly inside transient isolated worktrees; trunk `main` is physically immutable. |
| **4** | **The Single Source of Truth Law** | Unified Spacetime Metric | All physics telemetry binds strictly to compiled Go structs in `substrate/cli/physics.go`. |
| **5** | **The Zero-Hardcoded-Path Law** | Relativistic Reference Frames | Symbol coordinates and paths resolve dynamically relative to transient worktree roots. |
| **6** | **The Canonical Singularity Law** | Conservation of Energy | Exactly one canonical tensor for mass, drag, and collision; zero duplicate metrics or shadow state. |
| **7** | **The Fail-Closed Pre-Flight Law** | Atmospheric Re-entry Envelope | Any DoD gate failure triggers an immediate wave-off and restoration of the clean baseline (`git checkout -- <file>`). |

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
In software physics, a source file or package possesses **Inertia** not because of its byte count on disk, but because of its AST cardinality, decision branching, and topological coupling.

#### Mass Metric:
For any source file $f \in \mathcal{F}$, its physical mass $M(f)$ is defined as:

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

Where $\text{DuplicationRatio}(f) \in [0.0, 1.0]$ is computed via rolling Rabin-Karp hashes over sliding token windows of size $w = 10$.

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

When a RED collision is detected while agents are typing, the substrate alerts the orchestrator before either agent attempts to commit.

---

### 3.4 The 60Hz Telemetry Loop & Nyquist-Shannon Sampling
According to the Nyquist-Shannon sampling theorem, to prevent aliasing and control-loop divergence, the monitoring feedback loop must sample at more than twice the maximum frequency of environmental disturbance:

$$f_{\text{feedback}} > 2 \cdot f_{\text{mutation}}$$

If an autonomous coding agent generates AST mutations at 2Hz (tokens streaming into buffer), the substrate must evaluate AST invariants, type checking, and boundary constraints at sub-second intervals ($f \ge 4\text{Hz}$, targeting 60Hz frame intervals for telemetry streaming).

This high-frequency loop ensures the agent stays strictly within its **Controllable Basin of Attraction**, preventing compounding hallucinations.

---

### 3.5 Two-Phase Commit (2PC) Atmospheric Re-entry
Trunk branch `main` represents the pristine surface of production reality. Merging a transient worktree is modeled as an **atmospheric re-entry maneuver**:

1. **Pre-Flight Entry Vector:** Phase transitions from `EDIT` to `REVIEW`. CapBAC leases freeze further file mutations.
2. **The DoD Thermal Shield:** `nomos verify` runs all Definition of Done quality gates against the worktree AST.
3. **Aerodynamic Wave-Off (Revert Invariant):** If any single gate fails ($exit \ne 0$), the landing is aborted immediately. The worktree executes:
   ```bash
   git checkout -- <file>
   ```
   No scorched earth or damaged state touches `main`.
4. **Touchdown & Release Receipt:** If all DoD quality gates pass cleanly, atomic 2PC merges the worktree to `main` and writes a cryptographic release receipt to Git Notes (`refs/notes/nomos/releases`).

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
