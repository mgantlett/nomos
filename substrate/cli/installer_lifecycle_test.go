// Package cli verifies executable POSIX sh round-trip contracts for install.sh and uninstall.sh.
package cli

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

// installerArchiveFixture holds paths for a hermetic local release tarball test bundle.
type installerArchiveFixture struct {
	repoRoot     string
	compiledBin  string
	tarName      string
	tarPath      string
	validSums    string
	tamperedSums string
}

// buildInstallerArchiveFixture creates a deterministic release tar.gz and SHA-256 checksum manifests.
func buildInstallerArchiveFixture(t *testing.T) installerArchiveFixture {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	_ = ensureCompiledHookTestEnv(t)
	compiledBin := filepath.Join(compiledHookTestBinDir, "nomos")
	binBytes, err := os.ReadFile(compiledBin)
	if err != nil {
		t.Fatalf("read compiled nomos: %v", err)
	}

	tarName := fmt.Sprintf("nomos_%s_%s.tar.gz", installerOSToken(), installerArchToken())
	bundleDir := t.TempDir()
	tarPath := filepath.Join(bundleDir, tarName)
	writeGzipTarWithSingleBinary(t, tarPath, binBytes)

	tarBytes, err := os.ReadFile(tarPath)
	if err != nil {
		t.Fatalf("read tar.gz: %v", err)
	}
	digest := sha256.Sum256(tarBytes)
	validHex := hex.EncodeToString(digest[:])
	tamperedHex := strings.Repeat("0", 64)

	validSums := filepath.Join(bundleDir, "checksums.txt")
	if wErr := os.WriteFile(validSums, []byte(fmt.Sprintf("%s  %s\n", validHex, tarName)), 0600); wErr != nil {
		t.Fatalf("write checksums.txt: %v", wErr)
	}
	tamperedSums := filepath.Join(bundleDir, "checksums_tampered.txt")
	if wErr := os.WriteFile(tamperedSums, []byte(fmt.Sprintf("%s  %s\n", tamperedHex, tarName)), 0600); wErr != nil {
		t.Fatalf("write tampered checksums.txt: %v", wErr)
	}

	return installerArchiveFixture{
		repoRoot:     repoRoot,
		compiledBin:  compiledBin,
		tarName:      tarName,
		tarPath:      tarPath,
		validSums:    validSums,
		tamperedSums: tamperedSums,
	}
}

// writeGzipTarWithSingleBinary writes a minimal GoReleaser-compatible tar.gz containing nomos.
func writeGzipTarWithSingleBinary(t *testing.T, tarPath string, binBytes []byte) {
	t.Helper()
	outFile, err := os.Create(tarPath)
	if err != nil {
		t.Fatalf("create archive %s: %v", tarPath, err)
	}
	gzWriter := gzip.NewWriter(outFile)
	tarWriter := tar.NewWriter(gzWriter)

	hdr := &tar.Header{
		Name: "nomos",
		Mode: 0755,
		Size: int64(len(binBytes)),
	}
	if hErr := tarWriter.WriteHeader(hdr); hErr != nil {
		t.Fatalf("write tar header: %v", hErr)
	}
	if _, wErr := tarWriter.Write(binBytes); wErr != nil {
		t.Fatalf("write tar body: %v", wErr)
	}
	if cErr := tarWriter.Close(); cErr != nil {
		t.Fatalf("close tar writer: %v", cErr)
	}
	if gErr := gzWriter.Close(); gErr != nil {
		t.Fatalf("close gzip writer: %v", gErr)
	}
	if fErr := outFile.Close(); fErr != nil {
		t.Fatalf("close archive file: %v", fErr)
	}
}

func installerOSToken() string {
	if runtime.GOOS == "darwin" {
		return "Darwin"
	}
	return "Linux"
}

func installerArchToken() string {
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "x86_64"
}

// runPOSIXScript executes a shell script under sh with an isolated home and custom overrides.
func runPOSIXScript(t *testing.T, scriptPath, homeDir string, extraEnv ...string) (string, error) {
	t.Helper()
	shBin, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("POSIX sh not available")
	}
	cmd := exec.Command(shBin, scriptPath)
	baseEnv := []string{
		"HOME=" + homeDir,
		"PATH=" + os.Getenv("PATH"),
	}
	cmd.Env = append(baseEnv, extraEnv...)
	out, runErr := cmd.CombinedOutput()
	return string(out), runErr
}

// TestInstallerAndUninstallerPOSIXRoundTrip verifies fail-closed checksum rejection, SHA-256 verified
// installation, state-preserving default uninstall, and NOMOS_PURGE_DATA=1 full teardown under POSIX sh.
func TestInstallerAndUninstallerPOSIXRoundTrip(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("POSIX installer round-trip applies to Linux and Darwin")
	}
	fx := buildInstallerArchiveFixture(t)
	installScript := filepath.Join(fx.repoRoot, "install.sh")
	uninstallScript := filepath.Join(fx.repoRoot, "uninstall.sh")

	homeDir := t.TempDir()
	installDir := filepath.Join(homeDir, "custom-bin")
	nomosHome := filepath.Join(homeDir, constants.DirNomosHome)
	installedBin := filepath.Join(installDir, "nomos")

	badOut, badErr := runPOSIXScript(t, installScript, homeDir,
		"NOMOS_INSTALL_DIR="+installDir,
		"NOMOS_LOCAL_TAR="+fx.tarPath,
		"NOMOS_LOCAL_CHECKSUMS="+fx.tamperedSums,
	)
	if badErr == nil || !strings.Contains(badOut, "SHA-256 checksum mismatch") {
		t.Fatalf("expected tampered checksum to fail closed, err=%v out=%s", badErr, badOut)
	}
	if _, statErr := os.Stat(installedBin); !os.IsNotExist(statErr) {
		t.Fatalf("expected zero installed binary after tampered checksum, got err=%v", statErr)
	}

	okOut, okErr := runPOSIXScript(t, installScript, homeDir,
		"NOMOS_INSTALL_DIR="+installDir,
		"NOMOS_LOCAL_TAR="+fx.tarPath,
		"NOMOS_LOCAL_CHECKSUMS="+fx.validSums,
	)
	if okErr != nil {
		t.Fatalf("expected valid install.sh execution to succeed (%v):\n%s", okErr, okOut)
	}
	if !strings.Contains(okOut, "Verified SHA-256 checksum") || !strings.Contains(okOut, "Installation verified:") {
		t.Fatalf("expected SHA-256 lock and version verification output, got:\n%s", okOut)
	}
	if verOut, verErr := exec.Command(installedBin, "version").CombinedOutput(); verErr != nil {
		t.Fatalf("installed nomos version failed (%v): %s", verErr, string(verOut))
	}

	verifyUninstallerModes(t, installScript, uninstallScript, fx.compiledBin, homeDir, installDir, nomosHome)
}

// verifyUninstallerModes asserts default state-preserving uninstall and NOMOS_PURGE_DATA=1 teardown.
func verifyUninstallerModes(t *testing.T, installScript, uninstallScript, compiledBin, homeDir, installDir, nomosHome string) {
	t.Helper()
	installedBin := filepath.Join(installDir, "nomos")
	ledgerMarker := filepath.Join(nomosHome, "mneme.db")
	if err := os.MkdirAll(nomosHome, 0700); err != nil {
		t.Fatalf("mkdir nomosHome: %v", err)
	}
	if err := os.WriteFile(ledgerMarker, []byte("sqlite-sentinel"), 0600); err != nil {
		t.Fatalf("seed ledger marker: %v", err)
	}

	keepOut, keepErr := runPOSIXScript(t, uninstallScript, homeDir,
		"NOMOS_INSTALL_DIR="+installDir,
		"NOMOS_HOME="+nomosHome,
	)
	if keepErr != nil {
		t.Fatalf("default uninstall.sh failed (%v):\n%s", keepErr, keepOut)
	}
	if _, statErr := os.Stat(installedBin); !os.IsNotExist(statErr) {
		t.Fatalf("expected binary %s to be removed by uninstall.sh", installedBin)
	}
	if _, statErr := os.Stat(ledgerMarker); statErr != nil {
		t.Fatalf("expected default uninstall.sh to preserve NOMOS_HOME ledger (%v)", statErr)
	}

	if reOut, reErr := runPOSIXScript(t, installScript, homeDir,
		"NOMOS_INSTALL_DIR="+installDir,
		"NOMOS_LOCAL_BIN="+compiledBin,
	); reErr != nil {
		t.Fatalf("reinstall via NOMOS_LOCAL_BIN failed (%v):\n%s", reErr, reOut)
	}

	purgeOut, purgeErr := runPOSIXScript(t, uninstallScript, homeDir,
		"NOMOS_INSTALL_DIR="+installDir,
		"NOMOS_HOME="+nomosHome,
		"NOMOS_PURGE_DATA=1",
	)
	if purgeErr != nil {
		t.Fatalf("purge uninstall.sh failed (%v):\n%s", purgeErr, purgeOut)
	}
	if _, statErr := os.Stat(installedBin); !os.IsNotExist(statErr) {
		t.Fatalf("expected binary %s removed on purge uninstall", installedBin)
	}
	if _, statErr := os.Stat(nomosHome); !os.IsNotExist(statErr) {
		t.Fatalf("expected NOMOS_HOME %s purged when NOMOS_PURGE_DATA=1", nomosHome)
	}
}
