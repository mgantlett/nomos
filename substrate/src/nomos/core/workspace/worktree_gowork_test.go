package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

type goWorkTestCase struct {
	name        string
	rootGoMod   string
	subModules  map[string]string
	wantHeader  string
	wantUses    []string
	forbidToken string
}

func TestScaffoldGoWork_DynamicVersionAndTestdataExclusion(t *testing.T) {
	tests := []goWorkTestCase{
		{
			name:        "adopts higher go 1.27.3 directive from upstream repo",
			rootGoMod:   "module github.com/google/osv-scanner/v2\n\ngo 1.27.3\n",
			subModules:  map[string]string{"testdata": "module fixture\n\ngo 1.99.0\n"},
			wantHeader:  "go 1.27.3\n",
			wantUses:    []string{"\t."},
			forbidToken: "testdata",
		},
		{
			name:       "preserves lower go 1.24.1 directive without forcing 1.26.5",
			rootGoMod:  "module example.com/legacy\n\ngo 1.24.1\n",
			wantHeader: "go 1.24.1\n",
			wantUses:   []string{"\t."},
		},
		{
			name:      "picks highest version across multi-module workspace",
			rootGoMod: "module example.com/root\n\ngo 1.25.2\n",
			subModules: map[string]string{
				"sdk":    "module example.com/root/sdk\n\ngo 1.27.1\n",
				"vendor": "module vendored\n\ngo 1.99.0\n",
			},
			wantHeader:  "go 1.27.1\n",
			wantUses:    []string{"\t.", "\t./sdk"},
			forbidToken: "vendor",
		},
		{
			name:       "falls back to default version when go directive omitted",
			rootGoMod:  "module example.com/minimal\n",
			wantHeader: "go " + defaultGoWorkVersion + "\n",
			wantUses:   []string{"\t."},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runGoWorkCase(t, tc)
		})
	}
}

func runGoWorkCase(t *testing.T, tc goWorkTestCase) {
	t.Helper()
	wtDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(wtDir, "go.mod"), []byte(tc.rootGoMod), 0644); err != nil {
		t.Fatalf("write root go.mod: %v", err)
	}
	for dir, mod := range tc.subModules {
		subDir := filepath.Join(wtDir, dir)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "go.mod"), []byte(mod), 0644); err != nil {
			t.Fatalf("write %s/go.mod: %v", dir, err)
		}
	}

	mgr := &Manager{RepoRoot: wtDir}
	if err := mgr.scaffoldGoWork(wtDir); err != nil {
		t.Fatalf("scaffoldGoWork failed: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(wtDir, "go.work"))
	if err != nil {
		t.Fatalf("read go.work: %v", err)
	}
	got := string(raw)
	if !strings.HasPrefix(got, tc.wantHeader) {
		t.Errorf("go.work header mismatch: want prefix %q, got:\n%s", tc.wantHeader, got)
	}
	for _, u := range tc.wantUses {
		if !strings.Contains(got, u) {
			t.Errorf("expected go.work to contain %q, got:\n%s", u, got)
		}
	}
	if tc.forbidToken != "" && strings.Contains(got, tc.forbidToken) {
		t.Errorf("expected go.work to omit %q, got:\n%s", tc.forbidToken, got)
	}
}

func TestSiblingNomosReplace_ScopesStrictlyToNomosImporters(t *testing.T) {
	parent := t.TempDir()
	siblingSub := filepath.Join(parent, constants.AppName, constants.DirSubstrate)
	if err := os.MkdirAll(siblingSub, 0755); err != nil {
		t.Fatalf("mkdir sibling substrate: %v", err)
	}

	thirdPartyRepo := filepath.Join(parent, "external-oss")
	thirdPartyWT := filepath.Join(thirdPartyRepo, constants.DirWorktrees, "external-oss-SUB-1")
	if err := os.MkdirAll(thirdPartyWT, 0755); err != nil {
		t.Fatalf("mkdir thirdPartyWT: %v", err)
	}
	if err := os.WriteFile(filepath.Join(thirdPartyWT, "go.mod"), []byte("module example.com/oss\n\ngo 1.27.3\n"), 0644); err != nil {
		t.Fatalf("write third-party go.mod: %v", err)
	}
	if got := siblingNomosReplace(thirdPartyRepo, thirdPartyWT, []string{"\t."}); got != "" {
		t.Errorf("expected empty replace for non-nomos module, got %q", got)
	}

	pluginRepo := filepath.Join(parent, "my-plugin")
	pluginWT := filepath.Join(pluginRepo, constants.DirWorktrees, "my-plugin-SUB-1")
	if err := os.MkdirAll(pluginWT, 0755); err != nil {
		t.Fatalf("mkdir pluginWT: %v", err)
	}
	pluginMod := "module example.com/plugin\n\ngo 1.26.5\n\nrequire github.com/mgantlett/nomos/substrate v0.1.0\n"
	if err := os.WriteFile(filepath.Join(pluginWT, "go.mod"), []byte(pluginMod), 0644); err != nil {
		t.Fatalf("write plugin go.mod: %v", err)
	}
	if got := siblingNomosReplace(pluginRepo, pluginWT, []string{"\t."}); !strings.Contains(got, "replace github.com/mgantlett/nomos/substrate =>") {
		t.Errorf("expected sibling replace for nomos-importing plugin, got %q", got)
	}
}
