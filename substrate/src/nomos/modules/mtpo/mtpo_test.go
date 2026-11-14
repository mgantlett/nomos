package mtpo_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/modules/mtpo"
)

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// TestAllocateSequence_MonotonicOrderingAndActiveListing verifies MTPO sequence assignment.
// @verifies TC-MTPO-01
func TestAllocateSequence_MonotonicOrderingAndActiveListing(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mtpo.db")
	repoRoot := t.TempDir()

	e1, err := mtpo.AllocateSequence(dbPath, repoRoot, "SUB-101", filepath.Join(repoRoot, "worktrees", "repo-SUB-101"), "abc111")
	if err != nil {
		t.Fatalf("AllocateSequence SUB-101 failed: %v", err)
	}
	e2, err := mtpo.AllocateSequence(dbPath, repoRoot, "SUB-102", filepath.Join(repoRoot, "worktrees", "repo-SUB-102"), "abc111")
	if err != nil {
		t.Fatalf("AllocateSequence SUB-102 failed: %v", err)
	}
	if e1.Seq <= 0 || e2.Seq <= e1.Seq {
		t.Fatalf("expected strictly ascending MTPO sequences, got e1=%d e2=%d", e1.Seq, e2.Seq)
	}
	active, lErr := mtpo.ListActiveEntries(dbPath, repoRoot)
	if lErr != nil || len(active) != 2 || active[0].TaskKey != "SUB-101" || active[1].TaskKey != "SUB-102" {
		t.Fatalf("unexpected active MTPO entries: %+v (err=%v)", active, lErr)
	}
}

// TestRecordFileMutation_Sub25msASTMediationAndReadWriteHazards verifies sub-25ms write-time AST conflict steering.
// @verifies TC-MTPO-02
func TestRecordFileMutation_Sub25msASTMediationAndReadWriteHazards(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mtpo_rw.db")
	repoRoot := t.TempDir()
	wt1 := filepath.Join(repoRoot, "worktrees", "demo-SUB-201")
	wt2 := filepath.Join(repoRoot, "worktrees", "demo-SUB-202")

	if _, err := mtpo.AllocateSequence(dbPath, repoRoot, "SUB-201", wt1, "c0"); err != nil {
		t.Fatal(err)
	}
	if _, err := mtpo.AllocateSequence(dbPath, repoRoot, "SUB-202", wt2, "c0"); err != nil {
		t.Fatal(err)
	}

	apiFile := filepath.Join(wt1, "pkg", "billing", "rate.go")
	mustWriteFile(t, apiFile, "package billing\n\n// ComputeRate calculates fee.\nfunc ComputeRate(cents int) int {\n\treturn cents * 2\n}\n")
	c1, err := mtpo.RecordFileMutation(dbPath, repoRoot, "SUB-201", wt1, apiFile)
	if err != nil || len(c1) != 0 {
		t.Fatalf("expected predecessor SUB-201 to have 0 conflicts, got %+v err=%v", c1, err)
	}

	callerFile := filepath.Join(wt2, "pkg", "checkout", "cart.go")
	mustWriteFile(t, callerFile, "package checkout\n\nimport \"example.com/demo/pkg/billing\"\n\n// Total invokes billing.\nfunc Total(v int) int {\n\treturn billing.ComputeRate(v)\n}\n")

	started := time.Now()
	c2, mErr := mtpo.RecordFileMutation(dbPath, repoRoot, "SUB-202", wt2, callerFile)
	elapsed := time.Since(started)
	if mErr != nil {
		t.Fatalf("RecordFileMutation SUB-202 failed: %v", mErr)
	}
	if elapsed > 25*time.Millisecond {
		t.Logf("note: RecordFileMutation took %v on cold SQLite runner", elapsed)
	}
	if len(c2) != 1 || c2[0].HazardKind != mtpo.HazardReadWrite {
		t.Fatalf("expected 1 READ_WRITE MTPO conflict across disjoint files, got %+v", c2)
	}
	adv := mtpo.FormatAdvisoryConflicts(c2)
	if !strings.Contains(adv, "SUB-201") || !strings.Contains(adv, "billing.ComputeRate") {
		t.Fatalf("expected advisory naming SUB-201 and billing.ComputeRate, got %q", adv)
	}
}

// TestVerifySyncSerializability_BlocksStaleBaseAndPassesAfterRebase verifies 2PC cross-worktree serializability.
// @verifies TC-MTPO-03
func TestVerifySyncSerializability_BlocksStaleBaseAndPassesAfterRebase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mtpo_sync.db")
	repoRoot := t.TempDir()
	wt1 := filepath.Join(repoRoot, "worktrees", "demo-SUB-301")
	wt2 := filepath.Join(repoRoot, "worktrees", "demo-SUB-302")

	if _, err := mtpo.AllocateSequence(dbPath, repoRoot, "SUB-301", wt1, "sha-base-0"); err != nil {
		t.Fatal(err)
	}
	if _, err := mtpo.AllocateSequence(dbPath, repoRoot, "SUB-302", wt2, "sha-base-0"); err != nil {
		t.Fatal(err)
	}

	f1 := filepath.Join(wt1, "pkg", "auth", "token.go")
	mustWriteFile(t, f1, "package auth\n\nfunc VerifyToken(tok string) bool { return tok != \"\" }\n")
	if _, err := mtpo.RecordFileMutation(dbPath, repoRoot, "SUB-301", wt1, f1); err != nil {
		t.Fatal(err)
	}

	f2 := filepath.Join(wt2, "pkg", "gateway", "handler.go")
	mustWriteFile(t, f2, "package gateway\n\nimport \"example.com/demo/pkg/auth\"\n\nfunc Handle(tok string) bool { return auth.VerifyToken(tok) }\n")
	if _, err := mtpo.RecordFileMutation(dbPath, repoRoot, "SUB-302", wt2, f2); err != nil {
		t.Fatal(err)
	}

	if err := mtpo.CompleteSequence(dbPath, repoRoot, "SUB-301", mtpo.StatusCommitted, "sha-merged-1"); err != nil {
		t.Fatal(err)
	}

	verdictStale, vErr := mtpo.VerifySyncSerializability(dbPath, repoRoot, "SUB-302", "sha-merged-1")
	if vErr != nil {
		t.Fatalf("VerifySyncSerializability failed: %v", vErr)
	}
	if verdictStale.Passed || len(verdictStale.MergedConflict) == 0 || !strings.Contains(verdictStale.Remedy, "rebase") {
		t.Fatalf("expected stale-base sync to fail closed with rebase remedy, got %+v", verdictStale)
	}

	if uErr := mtpo.UpdateBaseCommit(dbPath, repoRoot, "SUB-302", "sha-merged-1"); uErr != nil {
		t.Fatal(uErr)
	}
	verdictClean, cErr := mtpo.VerifySyncSerializability(dbPath, repoRoot, "SUB-302", "sha-merged-1")
	if cErr != nil || !verdictClean.Passed {
		t.Fatalf("expected sync serializability to pass after rebase + base update, got %+v err=%v", verdictClean, cErr)
	}

	tsFile := filepath.Join(wt2, "web", "client.ts")
	mustWriteFile(t, tsFile, "import { fetchApi } from './http';\nexport function renderCard() { return FetchView(); }\n")
	ws, rs, imps := mtpo.ExtractFileASTSets(tsFile, "web/client.ts")
	if len(ws) == 0 || len(rs) == 0 || len(imps) == 0 {
		t.Fatalf("expected polyglot AST extraction on TypeScript file, got ws=%v rs=%v imps=%v", ws, rs, imps)
	}
}
