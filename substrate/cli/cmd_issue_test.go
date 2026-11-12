package cli

import (
	"bytes"
	"github.com/mgantlett/nomos/substrate/cli/diag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify"
	"github.com/mgantlett/nomos/substrate/src/nomos/modules/verify/vscope"
)

func TestSanitizeDiagnosticTextAndBranchTopology(t *testing.T) {
	fakeRepoRoot := filepath.Join(t.TempDir(), "acme-bank-core")
	fakePosixUserPath := "/" + strings.Join([]string{"Users", "jdoe", "vault", "cfg.yaml"}, "/")

	tests := []struct {
		name      string
		input     string
		repoRoot  string
		wantSub   string
		forbidSub string
	}{
		{
			name:      "empty",
			input:     "",
			wantSub:   "",
			forbidSub: "REDACTED",
		},
		{
			name:      "redacts github tokens",
			input:     "failed with token ghp_123456789012345678901234567890 in log",
			wantSub:   "failed with token [REDACTED_GH_TOKEN] in log",
			forbidSub: "ghp_12345",
		},
		{
			name:      "redacts cloud and api keys",
			input:     "keys sk-ant-api03-abcdefghijklmnopqrstuvwxyz123456789 and AKIAIOSFODNN7EXAMPLE",
			wantSub:   "keys [REDACTED_API_KEY] and [REDACTED_API_KEY]",
			forbidSub: "AKIAIOSFODNN7EXAMPLE",
		},
		{
			name:      "redacts bearer header and multi-scheme database URIs",
			input:     "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9 db=postgres://dbadmin:s3cr3tPass@db.internal:5432/app",
			wantSub:   "postgres://user:[REDACTED]@db.internal:5432/app",
			forbidSub: "s3cr3tPass",
		},
		{
			name:      "redacts repository root and posix home paths",
			input:     "parse error in " + filepath.Join(fakeRepoRoot, "internal", "wire.go") + " and " + fakePosixUserPath,
			repoRoot:  fakeRepoRoot,
			wantSub:   "<repo-root>/internal/wire.go and ~/vault/cfg.yaml",
			forbidSub: "acme-bank-core",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diag.SanitizeDiagnosticText(tt.input, tt.repoRoot)
			if tt.wantSub != "" && !strings.Contains(got, tt.wantSub) {
				t.Fatalf("sanitizeDiagnosticText(%q) = %q, want substring %q", tt.input, got, tt.wantSub)
			}
			if tt.forbidSub != "" && strings.Contains(got, tt.forbidSub) {
				t.Fatalf("sanitizeDiagnosticText(%q) leaked %q in %q", tt.input, tt.forbidSub, got)
			}
		})
	}

	if got := diag.SanitizeText("https://svc:topsecret@github.com/org/repo.git"); !strings.Contains(got, "https://user:[REDACTED]@github.com") {
		t.Fatalf("sanitizeText failed basic-auth scrub: %q", got)
	}

	branchCases := []struct {
		branch     string
		isWorktree bool
		want       string
	}{
		{"main", false, diag.BranchTopologyTrunk},
		{"feature/SUB-1840", false, diag.BranchTopologyWorktree},
		{"feature/acme-bank-wire-fix", false, diag.BranchTopologyCustom},
		{"HEAD", false, diag.BranchTopologyDetached},
		{"", false, diag.BranchTopologyUnknown},
		{"", true, diag.BranchTopologyWorktree},
	}
	for _, bc := range branchCases {
		if got := diag.ClassifyBranchTopology(bc.branch, bc.isWorktree); got != bc.want {
			t.Fatalf("classifyBranchTopology(%q, %v) = %q, want %q", bc.branch, bc.isWorktree, got, bc.want)
		}
	}
}

func TestIssueCmdOfflineDefaultAndNonInteractiveWebGuard(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("NOMOS_DIAGNOSTICS_CACHE_DIR", cacheDir)

	outDir := t.TempDir()
	reportPath := filepath.Join(outDir, ".tmp", "nomos-diagnostic.md")

	cmd := diag.NewIssueCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		"Gate timeout on monorepo",
		"--output", reportPath,
		"--body", "dsn postgres://alice:supersecret@localhost:5432/core",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("offline nomos issue failed: %v", err)
	}

	raw, rErr := os.ReadFile(reportPath)
	if rErr != nil {
		t.Fatalf("expected offline report file %s: %v", reportPath, rErr)
	}
	md := string(raw)
	if !strings.Contains(md, "postgres://user:[REDACTED]@localhost:5432/core") {
		t.Fatalf("expected scrubbed URI in saved markdown, got:\n%s", md)
	}
	if !strings.Contains(md, "Branch Topology:") || strings.Contains(md, "Active Branch:") {
		t.Fatalf("expected Branch Topology field instead of raw Active Branch, got:\n%s", md)
	}
	if !strings.Contains(md, diag.PrivateDiagnosticEmail) {
		t.Fatalf("expected private email channel in markdown, got:\n%s", md)
	}

	// TC-ISS-04 & TC-ISS-05: Non-interactive --web guard and openBrowser URL guard
	webCmd := diag.NewIssueCmd()
	var webOut bytes.Buffer
	webCmd.SetOut(&webOut)
	webCmd.SetArgs([]string{"Non-TTY check", "--output", reportPath, "--web"})
	if err := webCmd.Execute(); err != nil {
		t.Fatalf("unexpected error on non-TTY --web: %v", err)
	}
	if !strings.Contains(webOut.String(), "https://github.com/mgantlett/nomos/issues/new") {
		t.Fatalf("expected printed GitHub URL on non-TTY --web, got:\n%s", webOut.String())
	}
	if err := diag.OpenBrowser("https://evil.example.com/phish"); err == nil {
		t.Fatal("expected openBrowser to reject non-GitHub URL")
	}
}

func TestIssueCmdDryRunAndClear(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("NOMOS_DIAGNOSTICS_CACHE_DIR", cacheDir)

	cmd := diag.NewIssueCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"Test CLI dry-run", "--dry-run", "--clear"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error running nomos issue dry-run: %v", err)
	}
	if !strings.Contains(buf.String(), "Sanitized Markdown Payload:") {
		t.Fatalf("expected dry-run markdown output, got:\n%s", buf.String())
	}
}

func TestSubstrateAnomalyLedgerAndSyncAlert(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("NOMOS_DIAGNOSTICS_CACHE_DIR", cacheDir)
	repoRoot := filepath.Join(t.TempDir(), "client-secret-repo")

	failedReport := &verify.PipelineReport{
		Passed: false,
		GateResults: []*vscope.GateResult{
			{GateName: "AST Complexity Gate", Passed: false, DurationMicros: 1200},
		},
	}

	// 3 failures should NOT trigger gate_thrash yet (threshold is 4)
	for i := 0; i < 3; i++ {
		diag.RecordVerificationAnomalies(repoRoot, "SUB-42", "verify", failedReport, nil)
	}
	if got := diag.LoadRecentAnomalies(repoRoot); len(got) != 0 {
		t.Fatalf("expected 0 anomalies after 3 failures, got %d", len(got))
	}

	// 4th failure triggers gate_thrash; slow gate >=5s triggers gate_slow
	slowAndFailed := &verify.PipelineReport{
		Passed: false,
		GateResults: []*vscope.GateResult{
			{GateName: "AST Complexity Gate", Passed: false, DurationMicros: 5_400_000},
		},
	}
	diag.RecordVerificationAnomalies(repoRoot, "SUB-42", "verify", slowAndFailed, nil)

	recordRootExecutionAnomaly(
		repoRoot,
		types.NewNomosError(types.ErrCodeSubstrate, "sqlite busy in %s/state.db", repoRoot),
	)

	events := diag.LoadRecentAnomalies(repoRoot)
	if len(events) != 3 {
		t.Fatalf("expected 3 coalesced anomalies (slow, thrash, substrate_error), got %d: %+v", len(events), events)
	}
	for _, ev := range events {
		if strings.Contains(ev.Summary, "client-secret-repo") {
			t.Fatalf("anomaly summary leaked repo folder name: %q", ev.Summary)
		}
	}

	// First sync alert prints deterministic banner and marks Surfaced=true
	var syncBuf bytes.Buffer
	diag.RenderTaskSyncAnomalyAlert(&syncBuf, repoRoot, "SUB-42")
	alertOut := syncBuf.String()
	if !strings.Contains(alertOut, "Substrate Diagnostics:") || !strings.Contains(alertOut, "SUB-42") {
		t.Fatalf("expected deterministic sync anomaly alert, got:\n%s", alertOut)
	}

	// Second sync alert is silent because anomalies were already surfaced
	var secondBuf bytes.Buffer
	diag.RenderTaskSyncAnomalyAlert(&secondBuf, repoRoot, "SUB-42")
	if secondBuf.Len() != 0 {
		t.Fatalf("expected zero duplicate sync alert output, got:\n%s", secondBuf.String())
	}

	// Anomalies remain inspectable for nomos issue until --clear
	status := &diag.StatusReport{
		RepoRoot:       repoRoot,
		Phase:          "IDLE",
		ActiveBranch:   "feature/client-secret-branch",
		IsWorktree:     false,
		DatabaseStatus: "CONNECTED",
	}
	rep := diag.BuildIssueReportFromStatus(status, "Sync follow-up", "")
	md := diag.FormatIssueMarkdown(rep, diag.GenerateGitHubIssueURL(rep))
	if !strings.Contains(md, "Recent Local Substrate Anomalies") || !strings.Contains(md, "gate_thrash") {
		t.Fatalf("expected recent anomalies section in issue markdown, got:\n%s", md)
	}
	if strings.Contains(md, "client-secret-branch") {
		t.Fatalf("markdown leaked raw branch name:\n%s", md)
	}

	if err := diag.ClearSubstrateAnomalies(repoRoot); err != nil {
		t.Fatalf("ClearSubstrateAnomalies failed: %v", err)
	}
	if got := diag.LoadRecentAnomalies(repoRoot); len(got) != 0 {
		t.Fatalf("expected empty anomalies after clear, got %d", len(got))
	}
}
