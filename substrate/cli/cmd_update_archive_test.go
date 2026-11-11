package cli_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mgantlett/nomos/substrate/cli"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// @verifies TC-UPD-003
func TestVerifyArchiveChecksum_RejectsTamperedAndMissing(t *testing.T) {
	archiveBytes, validSum := buildTestTarGzArchive(t, "nomos", []byte("verified-binary"))
	gotSum, err := cli.VerifyArchiveChecksum("nomos_Linux_x86_64.tar.gz", archiveBytes, []byte(validSum+"  nomos_Linux_x86_64.tar.gz\n"))
	if err != nil || gotSum != validSum {
		t.Fatalf("expected valid checksum verification, got (%q, %v)", gotSum, err)
	}

	_, mismatchErr := cli.VerifyArchiveChecksum("nomos_Linux_x86_64.tar.gz", archiveBytes, []byte("deadbeef  nomos_Linux_x86_64.tar.gz\n"))
	var nErr *types.NomosError
	if !errors.As(mismatchErr, &nErr) || nErr.Code != types.ErrCodeValidationFailed {
		t.Fatalf("expected ERR_VALIDATION_FAILED on tampered checksum, got %v", mismatchErr)
	}

	_, missingErr := cli.VerifyArchiveChecksum("nomos_Darwin_arm64.tar.gz", archiveBytes, []byte(validSum+"  nomos_Linux_x86_64.tar.gz\n"))
	if missingErr == nil {
		t.Fatalf("expected missing archive entry in checksums.txt to fail closed")
	}

	if runtime.GOOS != "windows" {
		assetName, aErr := cli.FormatReleaseAssetName(runtime.GOOS, runtime.GOARCH)
		if aErr != nil {
			t.Fatalf("FormatReleaseAssetName: %v", aErr)
		}
		srv := startMockReleaseServer(t, "v99.5.0", assetName, archiveBytes, "0000000000000000000000000000000000000000000000000000000000000000")
		binPath := filepath.Join(t.TempDir(), "nomos")
		original := []byte("untouched-binary")
		if wErr := os.WriteFile(binPath, original, 0755); wErr != nil {
			t.Fatalf("WriteFile: %v", wErr)
		}
		t.Setenv(constants.EnvNomosHome, t.TempDir())
		t.Setenv(constants.EnvNomosBinaryPath, binPath)
		t.Setenv(constants.EnvNomosUpdateAPIURL, srv.URL+"/api/latest")
		t.Setenv(constants.EnvNomosUpdateBaseURL, srv.URL+"/dl")

		root := cli.NewRootCmd()
		root.SetOut(new(bytes.Buffer))
		root.SetArgs([]string{"update"})
		if exErr := root.Execute(); exErr == nil {
			t.Fatalf("expected nomos update to abort on SHA-256 mismatch")
		}
		after, rErr := os.ReadFile(binPath)
		if rErr != nil || !bytes.Equal(after, original) {
			t.Fatalf("existing binary must remain untouched on checksum failure")
		}
	}
}

func TestExtractNomosBinary_ZipAndAtomicReplace(t *testing.T) {
	var zBuf bytes.Buffer
	zw := zip.NewWriter(&zBuf)
	zf, err := zw.Create("nomos.exe")
	if err != nil {
		t.Fatalf("Create zip entry: %v", err)
	}
	want := []byte("MZ-windows-nomos")
	if _, wErr := zf.Write(want); wErr != nil {
		t.Fatalf("Write zip entry: %v", wErr)
	}
	if cErr := zw.Close(); cErr != nil {
		t.Fatalf("Close zip: %v", cErr)
	}

	got, eErr := cli.ExtractNomosBinary("nomos_Windows_x86_64.zip", zBuf.Bytes(), "windows")
	if eErr != nil || !bytes.Equal(got, want) {
		t.Fatalf("ExtractNomosBinary zip failed: err=%v got=%q", eErr, string(got))
	}

	target := filepath.Join(t.TempDir(), "sub", "nomos")
	if rErr := cli.AtomicReplaceBinary(target, want); rErr != nil {
		t.Fatalf("AtomicReplaceBinary: %v", rErr)
	}
	disk, dErr := os.ReadFile(target)
	if dErr != nil || !bytes.Equal(disk, want) {
		t.Fatalf("unexpected replaced binary: err=%v", dErr)
	}
}

