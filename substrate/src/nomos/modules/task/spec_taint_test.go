package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@nomos.local"},
		{"config", "user.name", "Nomos Test"},
	} {
		if out, err := workspace.RunSafeGit(dir, args...); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
	}
	return dir
}

func commitTempRepo(t *testing.T, dir, msg string) string {
	t.Helper()
	if out, err := workspace.RunSafeGit(dir, "add", "."); err != nil {
		t.Fatalf("git add: %v (%s)", err, string(out))
	}
	if out, err := workspace.RunSafeGit(dir, "commit", "-m", msg); err != nil {
		t.Fatalf("git commit: %v (%s)", err, string(out))
	}
	out, err := workspace.RunSafeGit(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// @verifies TC-OBL-004
// @verifies TC-OBL-005
func TestSpecDigestStampingAndStaleObligationTaint(t *testing.T) {
	repo := initTempGitRepo(t)
	specRel := filepath.Join("intent", "architecture", "specs", "auth_spec.md")
	specFull := filepath.Join(repo, specRel)
	srcRel := filepath.Join("substrate", "pkg", "auth", "guard.go")
	srcFull := filepath.Join(repo, srcRel)

	if err := os.MkdirAll(filepath.Dir(specFull), 0755); err != nil {
		t.Fatalf("mkdir spec: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(srcFull), 0755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(specFull, []byte("# Auth Spec v1\n"), 0644); err != nil {
		t.Fatalf("write spec v1: %v", err)
	}
	if err := os.WriteFile(srcFull, []byte("package auth\n// Enforce checks auth (@intent auth_spec.md).\nfunc Enforce() {}\n"), 0644); err != nil {
		t.Fatalf("write guard.go: %v", err)
	}
	sha1 := commitTempRepo(t, repo, "[SUB-10] feat(auth): initial implementation")

	digests := CollectReferencedSpecDigests(repo, []string{filepath.ToSlash(srcRel)}, "")
	if len(digests) != 1 || digests["intent/architecture/specs/auth_spec.md"] == "" {
		t.Fatalf("expected auth_spec.md digest collected, got: %v", digests)
	}

	ts := "2026-11-09T18:00:00Z"
	rec1 := ReleaseNotePayload{
		TaskKey:            "SUB-10",
		CommitSHA:          sha1,
		Timestamp:          ts,
		Phase:              "SYNC",
		DoDGatesPassed:     42,
		CapBACTokenID:      "cap-SUB-10",
		SpecDigests:        digests,
		WalkthroughContent: "verified v1",
		SHA256Receipt:      ComputeReleaseReceipt("SUB-10", sha1, ts, "cap-SUB-10"),
	}
	if err := RecordReleaseNote(repo, "", rec1); err != nil {
		t.Fatalf("RecordReleaseNote SUB-10: %v", err)
	}

	staleInit, err := ComputeStaleSpecObligations(repo, "")
	if err != nil || len(staleInit) != 0 {
		t.Fatalf("expected 0 stale obligations initially, got: %v (err=%v)", staleInit, err)
	}

	// Simulate INT-11 mutating auth_spec.md after SUB-10 receipt (TC-OBL-004)
	if err := os.WriteFile(specFull, []byte("# Auth Spec v2 (new obligation)\n"), 0644); err != nil {
		t.Fatalf("write spec v2: %v", err)
	}
	commitTempRepo(t, repo, "[INT-11] docs(specs): tighten auth obligation")

	staleAfterInt, err := ComputeStaleSpecObligations(repo, "")
	if err != nil || len(staleAfterInt) != 1 {
		t.Fatalf("expected 1 stale obligation after INT-11 spec edit, got: %v (err=%v)", staleAfterInt, err)
	}
	if staleAfterInt[0].SpecPath != "intent/architecture/specs/auth_spec.md" || staleAfterInt[0].LastVerifiedBy != "SUB-10" {
		t.Fatalf("unexpected stale obligation metadata: %+v", staleAfterInt[0])
	}

	// Simulate SUB-12 re-verifying against updated spec SHA-256 (TC-OBL-005)
	if err := os.WriteFile(srcFull, []byte("package auth\n// Enforce checks auth v2 (@intent auth_spec.md).\nfunc Enforce() {}\n"), 0644); err != nil {
		t.Fatalf("write guard.go v2: %v", err)
	}
	sha3 := commitTempRepo(t, repo, "[SUB-12] feat(auth): satisfy v2 auth obligation")
	digestsV2 := CollectReferencedSpecDigests(repo, nil, "INT-11")
	rec2 := ReleaseNotePayload{
		TaskKey:            "SUB-12",
		CommitSHA:          sha3,
		Timestamp:          ts,
		Phase:              "SYNC",
		DoDGatesPassed:     42,
		CapBACTokenID:      "cap-SUB-12",
		SpecDigests:        digestsV2,
		WalkthroughContent: "verified v2",
		SHA256Receipt:      ComputeReleaseReceipt("SUB-12", sha3, ts, "cap-SUB-12"),
	}
	if err := RecordReleaseNote(repo, "", rec2); err != nil {
		t.Fatalf("RecordReleaseNote SUB-12: %v", err)
	}

	staleCleared, err := ComputeStaleSpecObligations(repo, "")
	if err != nil || len(staleCleared) != 0 {
		t.Fatalf("expected stale obligation taint cleared after SUB-12 receipt, got: %v (err=%v)", staleCleared, err)
	}

	bad := rec2
	bad.SpecDigests = map[string]string{"intent/architecture/specs/auth_spec.md": "not-a-sha256"}
	if vErr := ValidateReleaseNote(bad); vErr == nil {
		t.Fatal("expected invalid spec_digests hex to fail ValidateReleaseNote")
	}
}

func TestTestOnlyVerifiesAndContractSpecDigests(t *testing.T) {
	repo := initTempGitRepo(t)
	specFull := filepath.Join(repo, "specs", "ledger_spec.md")
	pyTestFull := filepath.Join(repo, "tests", "test_ledger.py")
	if err := os.MkdirAll(filepath.Dir(specFull), 0755); err != nil {
		t.Fatalf("mkdir specs: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(pyTestFull), 0755); err != nil {
		t.Fatalf("mkdir tests: %v", err)
	}
	if err := os.WriteFile(specFull, []byte("# Ledger Spec\n| ID | Desc |\n|---|---|\n| `TC-LED-01` | Check |\n"), 0644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	if err := os.WriteFile(pyTestFull, []byte("# @verifies TC-LED-01\ndef test_ok():\n    assert True\n"), 0644); err != nil {
		t.Fatalf("write py test: %v", err)
	}
	digests := CollectReferencedSpecDigestsWithRoots(repo, []string{"tests/test_ledger.py"}, "", []string{"specs"})
	if digests["specs/ledger_spec.md"] == "" {
		t.Fatalf("expected test-only @verifies TC-LED-01 to resolve specs/ledger_spec.md digest, got: %v", digests)
	}
	contractDigests := CollectContractSpecDigests(repo, []string{"specs", "specs/ledger_spec.md"})
	if contractDigests["specs/ledger_spec.md"] == "" {
		t.Fatalf("expected CollectContractSpecDigests to hash specs/ledger_spec.md, got: %v", contractDigests)
	}
	directSpecDigests := CollectReferencedSpecDigestsWithRoots(repo, []string{"specs/ledger_spec.md"}, "", []string{"specs"})
	if directSpecDigests["specs/ledger_spec.md"] == "" {
		t.Fatalf("expected direct spec file path in touchedFiles to resolve digest, got: %v", directSpecDigests)
	}
}
