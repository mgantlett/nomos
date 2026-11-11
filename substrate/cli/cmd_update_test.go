package cli_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

func buildTestTarGzArchive(t *testing.T, binaryName string, payload []byte) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     binaryName,
		Mode:     0755,
		Size:     int64(len(payload)),
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatalf("Write tar payload: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("Close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("Close gzip: %v", err)
	}
	raw := buf.Bytes()
	sum := sha256.Sum256(raw)
	return raw, hex.EncodeToString(sum[:])
}

func startMockReleaseServer(t *testing.T, tag, assetName string, archiveBytes []byte, checksumHex string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/latest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"tag_name":%q}`, tag)
	})
	mux.HandleFunc("/dl/"+tag+"/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archiveBytes)
	})
	mux.HandleFunc("/dl/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s  %s\n", checksumHex, assetName)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// @verifies TC-UPD-001
func TestUpdateCmd_CanonicalCheckJSONAndZeroAlias(t *testing.T) {
	t.Setenv(constants.EnvNomosHome, t.TempDir())
	updCmd := cli.NewUpdateCmd()
	if len(updCmd.Aliases) != 0 {
		t.Fatalf("Law 6 violation: expected zero aliases on update command, got %v", updCmd.Aliases)
	}

	rootAliasCheck := cli.NewRootCmd()
	rootAliasCheck.SetOut(new(bytes.Buffer))
	rootAliasCheck.SetErr(new(bytes.Buffer))
	rootAliasCheck.SetArgs([]string{"upgrade"})
	if err := rootAliasCheck.Execute(); err == nil {
		t.Fatalf("expected non-canonical 'nomos upgrade' alias to be rejected")
	}

	srv := startMockReleaseServer(t, "v99.2.0", "nomos_Linux_x86_64.tar.gz", []byte("noop"), "00")
	binPath := filepath.Join(t.TempDir(), "bin", "nomos")
	t.Setenv(constants.EnvNomosBinaryPath, binPath)
	t.Setenv(constants.EnvNomosUpdateAPIURL, srv.URL+"/api/latest")

	root := cli.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"update", "--check", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("nomos update --check --json failed: %v", err)
	}

	var rep cli.UpdateReport
	if uErr := json.Unmarshal(out.Bytes(), &rep); uErr != nil {
		t.Fatalf("failed unmarshaling UpdateReport JSON: %v\nraw: %s", uErr, out.String())
	}
	if !rep.UpdateAvailable || !rep.CheckOnly || rep.Updated || rep.TargetVersion != "99.2.0" {
		t.Fatalf("unexpected UpdateReport: %+v", rep)
	}
}

// @verifies TC-UPD-002
func TestUpdateCmd_PackageManagerGuardBlocksOverwrite(t *testing.T) {
	t.Setenv(constants.EnvNomosHome, t.TempDir())
	srv := startMockReleaseServer(t, "v99.3.0", "nomos_Linux_x86_64.tar.gz", []byte("noop"), "00")
	t.Setenv(constants.EnvNomosUpdateAPIURL, srv.URL+"/api/latest")

	cases := []struct {
		binPath    string
		wantMethod string
		wantRemedy string
	}{
		{filepath.Join("/opt", "homebrew", "bin", "nomos"), "homebrew", "brew upgrade --cask mgantlett/tap/nomos"},
		{filepath.Join("/usr", "bin", "nomos"), "aur", "yay -Syu nomos-bin"},
		{filepath.Join("/nix", "store", "abc-nomos", "bin", "nomos"), "nix", "nix profile upgrade nomos"},
	}

	for _, tc := range cases {
		method, rem := cli.DetectInstallMethod(tc.binPath)
		if method != tc.wantMethod || rem != tc.wantRemedy {
			t.Fatalf("DetectInstallMethod(%s) = (%q, %q), want (%q, %q)", tc.binPath, method, rem, tc.wantMethod, tc.wantRemedy)
		}
		t.Setenv(constants.EnvNomosBinaryPath, tc.binPath)
		root := cli.NewRootCmd()
		root.SetOut(new(bytes.Buffer))
		root.SetArgs([]string{"update"})
		err := root.Execute()
		if err == nil {
			t.Fatalf("expected package-manager binary %s to block overwrite", tc.binPath)
		}
		var nErr *types.NomosError
		if !errors.As(err, &nErr) || nErr.Code != types.ErrCodeBoundary || !strings.Contains(nErr.Remedy, tc.wantRemedy) {
			t.Fatalf("expected ERR_BOUNDARY_VIOLATION with remedy %q, got %v", tc.wantRemedy, err)
		}
	}
}

func TestUpdateCmd_EndToEndStandaloneAtomicSwap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("tar.gz standalone fixture targets POSIX GOOS")
	}
	t.Setenv(constants.EnvNomosHome, t.TempDir())
	assetName, err := cli.FormatReleaseAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		t.Fatalf("FormatReleaseAssetName: %v", err)
	}
	newBinContent := []byte("#!/bin/sh\necho nomos-v99.4.0\n")
	archiveBytes, sumHex := buildTestTarGzArchive(t, "nomos", newBinContent)
	srv := startMockReleaseServer(t, "v99.4.0", assetName, archiveBytes, sumHex)

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "nomos")
	if wErr := os.WriteFile(binPath, []byte("old-binary"), 0755); wErr != nil {
		t.Fatalf("WriteFile: %v", wErr)
	}
	t.Setenv(constants.EnvNomosBinaryPath, binPath)
	t.Setenv(constants.EnvNomosUpdateAPIURL, srv.URL+"/api/latest")
	t.Setenv(constants.EnvNomosUpdateBaseURL, srv.URL+"/dl")

	root := cli.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"update", "--json"})
	if exErr := root.Execute(); exErr != nil {
		t.Fatalf("nomos update failed: %v", exErr)
	}

	var rep cli.UpdateReport
	if uErr := json.Unmarshal(out.Bytes(), &rep); uErr != nil {
		t.Fatalf("Unmarshal UpdateReport: %v", uErr)
	}
	if !rep.Updated || rep.ChecksumSHA256 != sumHex {
		t.Fatalf("expected verified update receipt, got %+v", rep)
	}
	replaced, rErr := os.ReadFile(binPath)
	if rErr != nil || !bytes.Equal(replaced, newBinContent) {
		t.Fatalf("binary was not atomically replaced: err=%v got=%q", rErr, string(replaced))
	}
}

