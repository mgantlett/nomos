package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mgantlett/nomos/substrate/cli"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

func TestVersionCmd_SurfacesCachedUpdateNotice(t *testing.T) {
	nomosHome := t.TempDir()
	t.Setenv(constants.EnvNomosHome, nomosHome)
	t.Setenv(constants.EnvNomosNoUpdateCheck, "")
	t.Setenv("CI", "")

	if err := cli.SaveUpdateCheckCache(cli.UpdateCheckCache{
		CheckedAt:     time.Now().UTC().Format(time.RFC3339),
		LatestVersion: "99.9.0",
	}); err != nil {
		t.Fatalf("SaveUpdateCheckCache: %v", err)
	}

	verCmd := cli.NewVersionCmd()
	var humanOut bytes.Buffer
	verCmd.SetOut(&humanOut)
	if err := verCmd.Execute(); err != nil {
		t.Fatalf("version execution failed: %v", err)
	}
	if !strings.Contains(humanOut.String(), "99.9.0") {
		t.Fatalf("expected human version output to include cached update notice, got %q", humanOut.String())
	}

	jsonCmd := cli.NewVersionCmd()
	var jsonOut bytes.Buffer
	jsonCmd.SetOut(&jsonOut)
	jsonCmd.SetArgs([]string{"--json"})
	if err := jsonCmd.Execute(); err != nil {
		t.Fatalf("version --json failed: %v", err)
	}
	var info cli.VersionInfo
	if uErr := json.Unmarshal(jsonOut.Bytes(), &info); uErr != nil {
		t.Fatalf("Unmarshal VersionInfo: %v", uErr)
	}
	if !info.UpdateAvailable || info.LatestVersion != "99.9.0" {
		t.Fatalf("expected VersionInfo update fields populated, got %+v", info)
	}
}

