// Package session verifies Sentinel CHARTER-4-BOUNDARY workspace authorization
// for Nomos plugin repositories and downstream consumer enforcement (@intent nomos_hermetic_worktree_spec.md).
package session

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEvaluateSentinelPluginWorkspaceAuthorization verifies that repositories containing
// substrate/cmd/nomos-plugin-* score 100 CLEAN when editing substrate/pkg/... and inspecting nomos/substrate.
//
// @verifies TC-SEN-010
func TestEvaluateSentinelPluginWorkspaceAuthorization(t *testing.T) {
	tempDir := t.TempDir()
	pluginRepo := filepath.Join(tempDir, "acme-plugin-workspace")
	pluginEntrypoint := filepath.Join(pluginRepo, "substrate", "cmd", "nomos-plugin-demo")
	wtRel := filepath.Join("worktrees", "acme-plugin-SUB-101")
	wtAbs := filepath.Join(pluginRepo, wtRel)
	pkgTarget := filepath.Join(wtAbs, "substrate", "pkg", "sovereign", "handler.go")
	upstreamSubstrate := filepath.Join(tempDir, "nomos", "substrate")

	for _, dir := range []string{pluginEntrypoint, filepath.Dir(pkgTarget), upstreamSubstrate} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create test fixture dir %s: %v", dir, err)
		}
	}

	detail := &SessionDetail{
		AgentSession: AgentSession{
			ID:           "sess-plugin-clean",
			WorktreePath: filepath.ToSlash(wtRel),
			Status:       "ACTIVE",
		},
	}
	lines := []string{
		`{"tool_calls":[{"name":"edit_file","args":{"Path":"` + filepath.ToSlash(pkgTarget) + `"}}]}`,
		`{"tool_calls":[{"name":"run_command","args":{"CommandLine":"ls ` + filepath.ToSlash(upstreamSubstrate) + `","Cwd":"` + filepath.ToSlash(wtAbs) + `"}}]}`,
	}

	sc := EvaluateSentinel(detail, lines, pluginRepo)
	assertBoundaryOutcome(t, sc, "CLEAN", 100, true)

	if got := detectPersona(&detail.AgentSession, pluginRepo); got != "SUBSTRATE_ARCHITECT" {
		t.Fatalf("expected plugin workspace persona SUBSTRATE_ARCHITECT, got %s", got)
	}
}

// TestEvaluateSentinelWorktreeOnlyPluginAuthorization verifies worktree-scoped plugin discovery
// when the plugin entrypoint is created inside an active task worktree.
//
// @verifies TC-SEN-010
func TestEvaluateSentinelWorktreeOnlyPluginAuthorization(t *testing.T) {
	tempDir := t.TempDir()
	pluginRepo := filepath.Join(tempDir, "acme-new-plugin")
	wtRel := filepath.Join("worktrees", "acme-new-plugin-SUB-202")
	wtPluginCmd := filepath.Join(pluginRepo, wtRel, "substrate", "cmd", "nomos-plugin-demo")
	upstreamSubstrate := filepath.Join(tempDir, "nomos", "substrate")

	if err := os.MkdirAll(wtPluginCmd, 0755); err != nil {
		t.Fatalf("failed to create worktree plugin dir: %v", err)
	}

	detail := &SessionDetail{
		AgentSession: AgentSession{
			ID:           "sess-wt-plugin",
			WorktreePath: filepath.ToSlash(wtRel),
			Status:       "ACTIVE",
		},
	}
	lines := []string{
		`{"tool_calls":[{"name":"read_file","args":{"Path":"` + filepath.ToSlash(filepath.Join(upstreamSubstrate, "go.mod")) + `"}}]}`,
	}

	sc := EvaluateSentinel(detail, lines, pluginRepo)
	assertBoundaryOutcome(t, sc, "CLEAN", 100, true)
}

// TestEvaluateSentinelDownstreamConsumerPeekingFlagged verifies that external downstream
// consumer workspaces lacking substrate/cmd/nomos* are flagged SUSPICIOUS when peeking.
//
// @verifies TC-SEN-010
func TestEvaluateSentinelDownstreamConsumerPeekingFlagged(t *testing.T) {
	tempDir := t.TempDir()
	consumerRepo := filepath.Join(tempDir, "acme-consumer-service")
	wtRel := filepath.Join("worktrees", "acme-consumer-SUB-303")
	if err := os.MkdirAll(filepath.Join(consumerRepo, wtRel, "internal", "api"), 0755); err != nil {
		t.Fatalf("failed to create consumer dir: %v", err)
	}

	upstreamFile := filepath.ToSlash(filepath.Join(tempDir, "nomos", "substrate", "cli", "root.go"))
	externalSovFile := filepath.ToSlash(filepath.Join(tempDir, "external-vendor", "substrate", "pkg", "sovereign", "types.go"))

	cases := []struct {
		name string
		line string
	}{
		{
			name: "peeks_into_upstream_nomos_substrate",
			line: `{"tool_calls":[{"name":"read_file","args":{"Path":"` + upstreamFile + `"}}]}`,
		},
		{
			name: "peeks_into_external_pkg_sovereign",
			line: `{"tool_calls":[{"name":"read_file","args":{"Path":"` + externalSovFile + `"}}]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			detail := &SessionDetail{
				AgentSession: AgentSession{
					ID:           "sess-" + tc.name,
					WorktreePath: filepath.ToSlash(wtRel),
					Status:       "ACTIVE",
				},
			}
			sc := EvaluateSentinel(detail, []string{tc.line}, consumerRepo)
			assertBoundaryOutcome(t, sc, "SUSPICIOUS", 80, false)
		})
	}
}

// TestEvaluateSentinelSelfOwnedRepoPackageAuthorization verifies that a workspace owning
// substrate/pkg/sovereign inside its own repoRoot passes for internal edits and fails on external escapes.
//
// @verifies TC-SEN-010
func TestEvaluateSentinelSelfOwnedRepoPackageAuthorization(t *testing.T) {
	tempDir := t.TempDir()
	ownerRepo := filepath.Join(tempDir, "acme-owned-suite")
	ownedFile := filepath.Join(ownerRepo, "substrate", "pkg", "sovereign", "service.go")
	externalFile := filepath.Join(tempDir, "other-suite", "substrate", "pkg", "sovereign", "secret.go")
	if err := os.MkdirAll(filepath.Dir(ownedFile), 0755); err != nil {
		t.Fatalf("failed to create owned package dir: %v", err)
	}

	detail := &SessionDetail{
		AgentSession: AgentSession{
			ID:           "sess-self-owned",
			WorktreePath: "worktrees/acme-owned-SUB-404",
			Status:       "ACTIVE",
		},
	}

	ownedLine := `{"tool_calls":[{"name":"edit_file","args":{"Path":"` + filepath.ToSlash(ownedFile) + `"}}]}`
	assertBoundaryOutcome(t, EvaluateSentinel(detail, []string{ownedLine}, ownerRepo), "CLEAN", 100, true)

	escapedLine := `{"tool_calls":[{"name":"read_file","args":{"Path":"` + filepath.ToSlash(externalFile) + `"}}]}`
	assertBoundaryOutcome(t, EvaluateSentinel(detail, []string{escapedLine}, ownerRepo), "SUSPICIOUS", 80, false)
}

// assertBoundaryOutcome asserts overall Sentinel status, score, and CHARTER-4-BOUNDARY invariant result.
func assertBoundaryOutcome(t *testing.T, sc *SentinelScorecard, wantStatus string, wantScore int, wantBoundaryPass bool) {
	t.Helper()
	if sc.IntegrityStatus != wantStatus || sc.OverallScore != wantScore {
		t.Fatalf("expected %s (%d), got %s (%d): violations=%v", wantStatus, wantScore, sc.IntegrityStatus, sc.OverallScore, sc.Violations)
	}
	for _, inv := range sc.Invariants {
		if inv.ID == "CHARTER-4-BOUNDARY" {
			if inv.Passed != wantBoundaryPass {
				t.Fatalf("expected CHARTER-4-BOUNDARY Passed=%v, got %v (evidence=%q)", wantBoundaryPass, inv.Passed, inv.Evidence)
			}
			return
		}
	}
	t.Fatalf("CHARTER-4-BOUNDARY invariant missing from scorecard: %+v", sc.Invariants)
}
