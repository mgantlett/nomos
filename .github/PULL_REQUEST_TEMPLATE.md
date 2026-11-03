<!-- 🛡️ NOMOS DUAL-LAYER PULL REQUEST TEMPLATE -->

## 📋 Task & Plane Classification
- **Task Key**: `[INT-<N>]` or `[SUB-<N>]` (or `[CHORE]`)
- **Target Plane**: Intent (`intent/`) | Substrate (`substrate/`)
- **Governing Law(s)**: Law 1 | Law 2 | Law 3 | Law 4 | Law 5 | Law 6 | Law 7

---

## 📐 Dual-Layer Specification

### **Architectural Rationale:**
<!-- Provide the high-level intent, motivation, and system context for this change. -->

### **Blast Radius & Contracts:**
<!-- List modified packages, exported Go contracts, JSON Schema impacts, and downstream dependencies. -->

### **Resolution Mechanics:**
<!-- Detail the exact deterministic algorithms, AST sensors, or data structure changes implemented. -->

---

## 🛡️ Statutory Definition of Done Checklist

Every Pull Request must satisfy the **7 Laws of Nomos** before trunk integration:

- [ ] **Law 1 (Dual-Core Inversion)**: Execution governed deterministically by Go substrate core under indirect control.
- [ ] **Law 2 (Non-Verbal Proof)**: `nomos verify` passes all **42 Definition of Done Quality Gates** with exit code 0.
- [ ] **Law 2 (Anti-Shim Gate)**: Zero stubs (`return nil // TODO`), empty compound blocks, or bypassed test assertions.
- [ ] **Law 3 (Hermetic Worktree)**: Changes authored strictly inside an isolated task worktree (`worktrees/<repo>-<task>`). Trunk `main` remained untouched during authoring.
- [ ] **Law 4 (SSoT & Parity)**: `nomos verify` confirms zero drift across compiled Go SSoT contracts and `nomos.schema.json`.
- [ ] **Law 5 (Zero-Hardcoded-Path)**: Zero hardcoded absolute paths, home directories, or workspace structures in source.
- [ ] **Law 6 (Canonical Singularity)**: Zero aliases, redundant flags, or duplicate schemas introduced.
- [ ] **Law 7 (Fail-Closed Pre-Flight)**: Pure 2-plane singularity observed (INT does not touch substrate; SUB does not touch intent).
