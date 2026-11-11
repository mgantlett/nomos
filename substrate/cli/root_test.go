package cli_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
)

func TestNewRootCmd_Registration(t *testing.T) {
	root := cli.NewRootCmd()
	if root == nil {
		t.Fatalf("expected non-nil root command")
	}

	expectedSubcommands := []string{"init", "status", "task", "verify", "handshake", "directive", "capabilities", "mem", "session", "issue", "update", "version"}
	for _, sub := range expectedSubcommands {
		cmd, _, err := root.Find([]string{sub})
		if err != nil || cmd == nil || cmd.Name() != sub {
			t.Errorf("missing or unresolvable subcommand %q: %v", sub, err)
		}
	}
}

func TestRootCmd_VersionExecution(t *testing.T) {
	root := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	if !strings.Contains(buf.String(), cli.RootCmdVersion) {
		t.Errorf("expected version output to contain %s, got %s", cli.RootCmdVersion, buf.String())
	}

	rootFlag := cli.NewRootCmd()
	flagBuf := new(bytes.Buffer)
	rootFlag.SetOut(flagBuf)
	rootFlag.SetArgs([]string{"--version"})
	if err := rootFlag.Execute(); err != nil {
		t.Fatalf("--version flag failed: %v", err)
	}
	if !strings.Contains(flagBuf.String(), cli.RootCmdVersion) {
		t.Errorf("expected --version output to contain %s, got %s", cli.RootCmdVersion, flagBuf.String())
	}
}

func TestRootCmd_TaskStatusExecution(t *testing.T) {
	root := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"task", "status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("task status command failed: %v", err)
	}
}

func TestRootCmd_HelpOutput(t *testing.T) {
	root := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"--help"})

	if err := root.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Nomos") {
		t.Errorf("help output missing Nomos banner: %s", output)
	}

	expectedGroups := []string{
		"Workspace & Lifecycle Commands:",
		"Quality & Invariant Verification Commands:",
		"Agent Boot & Cognitive Memory Commands:",
		"Diagnostics & System Commands:",
	}
	for _, grp := range expectedGroups {
		if !strings.Contains(output, grp) {
			t.Errorf("help output missing expected command group %q", grp)
		}
	}
}

func TestRootCmd_CommandGroups(t *testing.T) {
	root := cli.NewRootCmd()
	expectedCommandGroups := map[string]string{
		"init":         "lifecycle",
		"status":       "lifecycle",
		"task":         "lifecycle",
		"verify":       "quality",
		"handshake":    "agent",
		"directive":    "agent",
		"capabilities": "agent",
		"mem":          "agent",
		"session":      "agent",
		"issue":        "diagnostics",
		"update":       "diagnostics",
		"version":      "diagnostics",
	}

	for cmdName, expectedGroup := range expectedCommandGroups {
		cmd, _, err := root.Find([]string{cmdName})
		if err != nil || cmd == nil {
			t.Fatalf("subcommand %q not found: %v", cmdName, err)
		}
		if cmd.GroupID != expectedGroup {
			t.Errorf("command %q has GroupID %q, expected %q", cmdName, cmd.GroupID, expectedGroup)
		}
	}
}

func TestRootCmd_StatusExecution(t *testing.T) {
	root := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("status command failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Nomos Operational Status") {
		t.Errorf("status output missing Nomos banner: %s", output)
	}
	if !strings.Contains(output, "Remote Sync") {
		t.Errorf("status output missing Remote Sync: %s", output)
	}
}

func TestRootCmd_ZeroFootprintForVersionAndHook(t *testing.T) {
	tmpHome := t.TempDir()
	tmpXDG := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CACHE_HOME", tmpXDG)

	verCmd := cli.NewRootCmd()
	verCmd.SetOut(new(bytes.Buffer))
	verCmd.SetArgs([]string{"version"})
	if err := verCmd.Execute(); err != nil {
		t.Fatalf("version failed: %v", err)
	}

	hookCmd := cli.NewRootCmd()
	hookCmd.SetIn(strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls -la"}}`))
	hookCmd.SetOut(new(bytes.Buffer))
	hookCmd.SetArgs([]string{"hook", "pre-tool", "--container-sandbox"})
	if err := hookCmd.Execute(); err != nil {
		t.Fatalf("hook pre-tool failed: %v", err)
	}

	homeEntries, err := os.ReadDir(tmpHome)
	if err != nil {
		t.Fatalf("ReadDir(tmpHome) failed: %v", err)
	}
	if len(homeEntries) != 0 {
		t.Fatalf("expected zero HOME files for version and hook commands, found %d", len(homeEntries))
	}
}
