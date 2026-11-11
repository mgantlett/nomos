package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

// @verifies TC-UPD-005
func TestWorkspaceContractSync_DetectsDriftAndRefreshesIdempotently(t *testing.T) {
	t.Setenv(constants.EnvNomosHome, t.TempDir())
	repoDir := t.TempDir()

	initCmd := cli.NewRootCmd()
	initCmd.SetOut(new(bytes.Buffer))
	initCmd.SetArgs([]string{"init", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("nomos init failed: %v", err)
	}

	if cleanDrift := cli.DetectWorkspaceContractDrift(repoDir); len(cleanDrift) != 0 {
		t.Fatalf("expected zero drift immediately after nomos init, got %v", cleanDrift)
	}

	cfgPath := filepath.Join(repoDir, constants.FileNomosYAML)
	cfgBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile nomos.yaml: %v", err)
	}
	staleCfg := strings.Replace(string(cfgBytes), "v"+cli.RootCmdVersion, "v4.1.0", 1)
	if wErr := os.WriteFile(cfgPath, []byte(staleCfg), 0644); wErr != nil {
		t.Fatalf("WriteFile stale nomos.yaml: %v", wErr)
	}

	agentsPath := filepath.Join(repoDir, "AGENTS.md")
	staleAgents := "<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->\nstale protocol\n<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->\n\n## Custom Team Rule\nKeep local notes.\n"
	if wErr := os.WriteFile(agentsPath, []byte(staleAgents), 0644); wErr != nil {
		t.Fatalf("WriteFile stale AGENTS.md: %v", wErr)
	}

	claudePath := filepath.Join(repoDir, "CLAUDE.md")
	legacyClaude := "<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->\nlegacy duplicated block\n<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->\n\n---\n\n## Custom Claude Note\nPrefer concise diffs.\n"
	if wErr := os.WriteFile(claudePath, []byte(legacyClaude), 0644); wErr != nil {
		t.Fatalf("WriteFile legacy CLAUDE.md: %v", wErr)
	}

	hookScript := filepath.Join(repoDir, constants.DirAgents, "scripts", "session-start.sh")
	if wErr := os.WriteFile(hookScript, []byte("#!/bin/sh\necho outdated\n"), 0755); wErr != nil {
		t.Fatalf("WriteFile stale hook script: %v", wErr)
	}

	drift := cli.DetectWorkspaceContractDrift(repoDir)
	if len(drift) < 4 {
		t.Fatalf("expected nomos.yaml, AGENTS.md, CLAUDE.md, and hook script drift, got %v", drift)
	}
	if notice := cli.FormatWorkspaceDriftNotice(repoDir); !strings.Contains(notice, "--sync-workspace") {
		t.Fatalf("expected actionable drift notice, got %q", notice)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"tag_name":%q}`, "v"+cli.RootCmdVersion)
	}))
	t.Cleanup(srv.Close)
	t.Setenv(constants.EnvNomosUpdateAPIURL, srv.URL)
	t.Setenv(constants.EnvNomosBinaryPath, filepath.Join("/opt", "homebrew", "bin", "nomos"))

	updCmd := cli.NewRootCmd()
	var out bytes.Buffer
	updCmd.SetOut(&out)
	updCmd.SetArgs([]string{"update", "--sync-workspace", "--dir", repoDir, "--json"})
	if exErr := updCmd.Execute(); exErr != nil {
		t.Fatalf("nomos update --sync-workspace failed: %v", exErr)
	}

	var rep cli.UpdateReport
	if uErr := json.Unmarshal(out.Bytes(), &rep); uErr != nil {
		t.Fatalf("Unmarshal UpdateReport: %v", uErr)
	}
	if !rep.WorkspaceSynchronized || rep.WorkspaceDrift {
		t.Fatalf("expected synchronized workspace with zero remaining drift, got %+v", rep)
	}

	refreshedAgents, rErr := os.ReadFile(agentsPath)
	if rErr != nil || !strings.Contains(string(refreshedAgents), "Custom Team Rule") || strings.Contains(string(refreshedAgents), "stale protocol") {
		t.Fatalf("expected AGENTS.md protocol refreshed while preserving custom rules")
	}
	refreshedClaude, clErr := os.ReadFile(claudePath)
	if clErr != nil || !strings.Contains(string(refreshedClaude), "@AGENTS.md") || !strings.Contains(string(refreshedClaude), "Custom Claude Note") || strings.Contains(string(refreshedClaude), "legacy duplicated block") {
		t.Fatalf("expected CLAUDE.md collapsed to @AGENTS.md while preserving custom Claude note, got %q", string(refreshedClaude))
	}
	refreshedCfg, cErr := os.ReadFile(cfgPath)
	if cErr != nil || !strings.Contains(string(refreshedCfg), "v"+cli.RootCmdVersion+"/nomos.schema.json") {
		t.Fatalf("expected nomos.yaml schema pin refreshed to v%s", cli.RootCmdVersion)
	}
}

