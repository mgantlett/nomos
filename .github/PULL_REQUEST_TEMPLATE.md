<!-- 🛡️ NOMOS PULL REQUEST TEMPLATE -->

## 📋 Summary & Plane Classification
- **Target Plane**: Intent (`intent/`) | Substrate (`substrate/`) | paired (`INT-<N>` + `SUB-<N>`)
- **Task Key (Optional for External PRs)**: `INT-<N>` or `SUB-<N>`
- **Governing Law(s)**: Law 1 | Law 2 | Law 3 | Law 4 | Law 5 | Law 6 | Law 7

---

## 🔍 Context & Changes
<!-- Describe the problem solved, modified packages/contracts, and verification coverage in concise prose. -->

---

## 🛡️ Statutory Definition of Done Checklist

Every Pull Request must satisfy the **7 Laws of Nomos** before trunk integration:

- [ ] **Law 1 (Dual-Core Inversion)**: Execution governed deterministically by Go substrate core under indirect control.
- [ ] **Law 2 (Non-Verbal Proof)**: `go -C substrate test ./...` and `nomos verify --all --strict` pass all **42 Definition of Done Quality Gates** with exit code `0`.
- [ ] **Law 2 (Anti-Shim Gate)**: Zero stubs (`return nil // TODO`), empty compound blocks, or bypassed test assertions.
- [ ] **Law 3 (Hermetic Worktree)**: Changes authored cleanly without mutating protected `main` directly.
- [ ] **Law 4 (SSoT & Parity)**: Zero drift across compiled Go SSoT contracts, specifications, and `nomos.schema.json`.
- [ ] **Law 5 (Zero-Hardcoded-Path)**: Zero hardcoded absolute paths, home directories, or workstation paths in source.
- [ ] **Law 6 (Canonical Singularity)**: Zero command aliases, redundant flags, duplicate schemas, or filesystem symlinks introduced.
- [ ] **Law 7 (Fail-Closed Pre-Flight)**: Plane boundaries, secret shields, and test coverage floors satisfied.
- [ ] **Provenance (DCO 1.1 & Apache 2.0 §5)**: Commits include a `Signed-off-by:` trailer (`git commit -s`) certifying right to contribute under the Apache License, Version 2.0.
