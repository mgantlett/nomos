package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
)

func TestCapabilitiesCmd_Default(t *testing.T) {
	root := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"capabilities"})

	if err := root.Execute(); err != nil {
		t.Fatalf("capabilities command failed: %v", err)
	}

	output := buf.String()
	expectedSections := []string{
		"Nomos Substrate & Intent Capabilities Catalog",
		"Ambient Lifecycle Hooks",
		"JIT Phase Directives",
		"Architectural Engines",
		"Invariant Quality Gates (7x3 DoD)",
		"Core CLI Commands",
	}

	for _, sec := range expectedSections {
		if !strings.Contains(output, sec) {
			t.Errorf("output missing expected section %q:\n%s", sec, output)
		}
	}
}

func TestCapabilitiesCmd_JSON(t *testing.T) {
	root := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"capabilities", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("capabilities --json failed: %v", err)
	}

	var catalog cli.CapabilitiesCatalog
	if err := json.Unmarshal(buf.Bytes(), &catalog); err != nil {
		t.Fatalf("failed unmarshaling capabilities JSON: %v\nOutput: %s", err, buf.String())
	}

	if catalog.TotalCount == 0 || len(catalog.Items) == 0 {
		t.Fatalf("expected non-empty capabilities items, got %d", catalog.TotalCount)
	}

	if catalog.HooksCount < 3 {
		t.Errorf("expected at least 3 ambient hooks, got %d", catalog.HooksCount)
	}
	if catalog.DirectivesCount < 5 {
		t.Errorf("expected at least 5 directives, got %d", catalog.DirectivesCount)
	}
	if catalog.EnginesCount < 7 {
		t.Errorf("expected at least 7 engines, got %d", catalog.EnginesCount)
	}
	if catalog.GatesCount != 21 {
		t.Errorf("expected exactly 21 invariant cells, got %d", catalog.GatesCount)
	}
	if catalog.CLICount < 10 {
		t.Errorf("expected at least 10 CLI commands, got %d", catalog.CLICount)
	}

	foundSessionHook := false
	foundSQLiteEngine := false
	foundGate := false

	for _, item := range catalog.Items {
		if item.ID == "hook-session-handshake" && item.Invocation == "SessionStart" {
			foundSessionHook = true
		}
		if item.Type == "engine" && strings.Contains(item.Name, "SQLite") {
			foundSQLiteEngine = true
		}
		if item.ID == "cell_1_1" {
			foundGate = true
		}
	}

	if !foundSessionHook {
		t.Errorf("expected hook-session-handshake with invocation SessionStart in items")
	}
	if !foundSQLiteEngine {
		t.Errorf("expected SQLite engine in items")
	}
	if !foundGate {
		t.Errorf("expected cell_1_1 gate in items")
	}
}

func TestCapabilitiesCmd_FilterTypes(t *testing.T) {
	testCases := []struct {
		filterType   string
		expectedType string
	}{
		{"hook", "hook"},
		{"directive", "directive"},
		{"engine", "engine"},
		{"gate", "gate"},
		{"cli", "cli"},
	}

	for _, tc := range testCases {
		t.Run(tc.filterType, func(t *testing.T) {
			root := cli.NewRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetArgs([]string{"capabilities", "--type", tc.filterType, "--json"})

			if err := root.Execute(); err != nil {
				t.Fatalf("capabilities --type %s failed: %v", tc.filterType, err)
			}

			var catalog cli.CapabilitiesCatalog
			if err := json.Unmarshal(buf.Bytes(), &catalog); err != nil {
				t.Fatalf("failed unmarshaling filtered JSON: %v", err)
			}

			if len(catalog.Items) == 0 {
				t.Fatalf("expected filtered items for type %q, got 0", tc.filterType)
			}

			for _, item := range catalog.Items {
				if item.Type != tc.expectedType {
					t.Errorf("item %s has type %q, expected %q", item.ID, item.Type, tc.expectedType)
				}
			}
		})
	}
}

func TestCapabilitiesCmd_InvalidFilter(t *testing.T) {
	root := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"capabilities", "--type", "invalid_type"})

	err := root.Execute()
	if err == nil {
		t.Fatalf("expected error for invalid capability filter, got nil")
	}
	if !strings.Contains(err.Error(), "unknown capability type") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCapabilitiesCmd_StandaloneEngineGrounding(t *testing.T) {
	tempDir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	if chErr := os.Chdir(tempDir); chErr != nil {
		t.Fatalf("failed to chdir to tempDir: %v", chErr)
	}
	defer func() {
		if revertErr := os.Chdir(origWd); revertErr != nil {
			t.Logf("warning: failed reverting working directory: %v", revertErr)
		}
	}()

	root := cli.NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"capabilities", "--type", "engine", "--json"})

	if execErr := root.Execute(); execErr != nil {
		t.Fatalf("capabilities --type engine failed in standalone mode: %v", execErr)
	}

	var catalog cli.CapabilitiesCatalog
	if unmarshalErr := json.Unmarshal(buf.Bytes(), &catalog); unmarshalErr != nil {
		t.Fatalf("failed unmarshaling catalog JSON: %v", unmarshalErr)
	}

	if len(catalog.Items) == 0 {
		t.Fatalf("expected engine items, got 0")
	}

	for _, it := range catalog.Items {
		if it.Type == "engine" && it.Status != "GROUNDED" {
			t.Errorf("engine %s in standalone binary mode has status %q, expected GROUNDED", it.ID, it.Status)
		}
	}
}

func TestCapabilitiesCmd_SingleItemDrillDown(t *testing.T) {
	cases := []struct {
		name      string
		arg       string
		wantID    string
		wantLaw   int
		wantError bool
	}{
		{name: "probe ID lookup", arg: "plane_separation", wantID: "plane_separation", wantLaw: 7},
		{name: "cell ID lookup", arg: "cell_2_1", wantID: "cell_2_1", wantLaw: 2},
		{name: "hook shorthand lookup", arg: "worktree-boundary-guard", wantID: "hook-worktree-boundary-guard", wantLaw: 0},
		{name: "unknown capability fails closed", arg: "nonexistent_probe_999", wantError: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := cli.NewRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"capabilities", tc.arg, "--json"})

			err := root.Execute()
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tc.arg)
				}
				return
			}
			if err != nil {
				t.Fatalf("capabilities %q --json failed: %v", tc.arg, err)
			}

			var detail cli.CapabilityDetail
			if uErr := json.Unmarshal(buf.Bytes(), &detail); uErr != nil {
				t.Fatalf("failed unmarshaling detail JSON: %v\n%s", uErr, buf.String())
			}
			if detail.ID != tc.wantID {
				t.Errorf("expected ID %q, got %q", tc.wantID, detail.ID)
			}
			if detail.Law != tc.wantLaw {
				t.Errorf("expected Law %d, got %d", tc.wantLaw, detail.Law)
			}
			if tc.wantLaw > 0 && strings.TrimSpace(detail.Remedy) == "" {
				t.Errorf("expected non-empty remedy for gate/probe %q", tc.arg)
			}
		})
	}
}
