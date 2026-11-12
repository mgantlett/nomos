// Package selfupdate implements verified pure-Go archive download, SHA-256 attestation, and atomic binary replacement.
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mgantlett/nomos/substrate/cli/tui"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

const (
	defaultGitHubRepo      = "mgantlett/nomos"
	maxArchiveDownloadSize = 128 << 20 // 128 MiB
	maxBinaryExtractSize   = 256 << 20 // 256 MiB
)

// ResolveLatestReleaseTag queries the latest published release tag via HTTP redirect or API JSON fallback.
// @intent task_lifecycle_cli_spec.md #UpdateCheckCache
func ResolveLatestReleaseTag(ctx context.Context) (string, error) {
	if strings.TrimSpace(os.Getenv(constants.EnvNomosUpdateAPIURL)) == "" {
		if redirected, rErr := resolveLatestTagViaRedirect(ctx); rErr == nil && redirected != "" {
			return redirected, nil
		}
	}
	apiURL := resolveReleaseAPIURL()
	body, err := fetchBoundedBytes(ctx, apiURL, 1<<20)
	if err != nil {
		return "", types.NewNomosError(types.ErrCodeSubstrate, "failed querying latest release metadata (%s): %v", apiURL, err)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if uErr := json.Unmarshal(body, &payload); uErr != nil {
		return "", types.NewNomosError(types.ErrCodeSubstrate, "failed decoding release metadata JSON: %v", uErr)
	}
	return NormalizeReleaseTag(payload.TagName)
}

func resolveLatestTagViaRedirect(ctx context.Context) (string, error) {
	latestURL := fmt.Sprintf("https://github.com/%s/releases/latest", resolveTargetGitHubRepo())
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, latestURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "nomos-cli/"+tui.ResolveAppVersion())
	noRedirect := &http.Client{
		Timeout:       updateHTTPTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		return "", err
	}
	if cErr := resp.Body.Close(); cErr != nil {
		// redirect response closed
	}
	loc := strings.TrimSpace(resp.Header.Get("Location"))
	idx := strings.Index(loc, "/releases/tag/")
	if idx < 0 {
		return "", fmt.Errorf("missing release tag redirect location")
	}
	tagPart := strings.Trim(loc[idx+len("/releases/tag/"):], "/")
	if qIdx := strings.IndexByte(tagPart, '?'); qIdx >= 0 {
		tagPart = tagPart[:qIdx]
	}
	return NormalizeReleaseTag(tagPart)
}

func resolveReleaseAPIURL() string {
	if custom := strings.TrimSpace(os.Getenv(constants.EnvNomosUpdateAPIURL)); custom != "" {
		return custom
	}
	return fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", resolveTargetGitHubRepo())
}

func resolveReleaseDownloadBaseURL(tag string) string {
	if custom := strings.TrimRight(strings.TrimSpace(os.Getenv(constants.EnvNomosUpdateBaseURL)), "/"); custom != "" {
		return fmt.Sprintf("%s/%s", custom, tag)
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s", resolveTargetGitHubRepo(), tag)
}

func resolveTargetGitHubRepo() string {
	if repo := strings.TrimSpace(os.Getenv("NOMOS_REPO")); repo != "" {
		return repo
	}
	return defaultGitHubRepo
}

func downloadVerifyAndReplaceBinary(ctx context.Context, tag, binaryPath string) (string, error) {
	assetName, err := FormatReleaseAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	baseURL := resolveReleaseDownloadBaseURL(tag)
	archiveBytes, aErr := fetchBoundedBytes(ctx, baseURL+"/"+assetName, maxArchiveDownloadSize)
	if aErr != nil {
		return "", types.NewNomosError(types.ErrCodeSubstrate, "failed downloading release archive %s: %v", assetName, aErr)
	}
	checksumBytes, cErr := fetchBoundedBytes(ctx, baseURL+"/checksums.txt", 1<<20)
	if cErr != nil {
		return "", types.NewNomosError(types.ErrCodeSubstrate, "failed downloading checksums.txt for %s: %v", tag, cErr)
	}
	sum, vErr := VerifyArchiveChecksum(assetName, archiveBytes, checksumBytes)
	if vErr != nil {
		return "", vErr
	}
	binBytes, eErr := ExtractNomosBinary(assetName, archiveBytes, runtime.GOOS)
	if eErr != nil {
		return "", eErr
	}
	if rErr := AtomicReplaceBinary(binaryPath, binBytes); rErr != nil {
		return "", rErr
	}
	clearDarwinQuarantineBestEffort(binaryPath)
	return sum, nil
}

func fetchBoundedBytes(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	if strings.HasPrefix(rawURL, "file://") {
		return readBoundedFileURL(rawURL, maxBytes)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "nomos-cli/"+tui.ResolveAppVersion())
	attachGitHubAuthIfAvailable(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := resp.Body.Close(); cErr != nil {
			// response body closed
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBytes))
}

func attachGitHubAuthIfAvailable(req *http.Request) {
	if req == nil || req.URL == nil || !strings.EqualFold(req.URL.Hostname(), "api.github.com") {
		return
	}
	tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if tok == "" {
		tok = strings.TrimSpace(os.Getenv("GH_TOKEN"))
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
}

func readBoundedFileURL(rawURL string, maxBytes int64) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "file" || parsed.Path == "" {
		return nil, fmt.Errorf("invalid file URL %q", rawURL)
	}
	f, err := os.Open(filepath.FromSlash(parsed.Path))
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := f.Close(); cErr != nil {
			// local fixture closed
		}
	}()
	return io.ReadAll(io.LimitReader(f, maxBytes))
}

// VerifyArchiveChecksum validates archiveBytes against checksums.txt and returns the hex SHA-256 digest.
// @intent task_lifecycle_cli_spec.md #UpdateReport
func VerifyArchiveChecksum(archiveName string, archiveBytes, checksumBytes []byte) (string, error) {
	expected := lookupChecksumEntry(archiveName, string(checksumBytes))
	if expected == "" {
		return "", types.NewNomosError(types.ErrCodeValidationFailed, "archive %q missing from checksums.txt", archiveName)
	}
	rawSum := sha256.Sum256(archiveBytes)
	actual := hex.EncodeToString(rawSum[:])
	if !strings.EqualFold(actual, expected) {
		return "", types.NewNomosError(
			types.ErrCodeValidationFailed,
			"SHA-256 checksum mismatch for %s (expected %s, got %s)",
			archiveName, expected, actual,
		)
	}
	return actual, nil
}

func lookupChecksumEntry(archiveName, content string) string {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") == archiveName {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

// ExtractNomosBinary extracts the compiled nomos binary from a .tar.gz or .zip release archive.
// @intent task_lifecycle_cli_spec.md #UpdateReport
func ExtractNomosBinary(archiveName string, archiveBytes []byte, goos string) ([]byte, error) {
	targetBase := "nomos"
	if strings.EqualFold(goos, "windows") {
		targetBase = "nomos.exe"
	}
	if strings.HasSuffix(archiveName, ".zip") {
		return extractBinaryFromZip(archiveBytes, targetBase)
	}
	return extractBinaryFromTarGz(archiveBytes, targetBase)
}

func extractBinaryFromTarGz(archiveBytes []byte, targetBase string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archiveBytes))
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "invalid gzip archive: %v", err)
	}
	defer func() {
		if cErr := gz.Close(); cErr != nil {
			// closed gzip stream
		}
	}()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, types.NewNomosError(types.ErrCodeValidationFailed, "corrupt tar archive: %v", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == targetBase {
			return readBoundedEntry(tr, targetBase)
		}
	}
	return nil, types.NewNomosError(types.ErrCodeValidationFailed, "binary %q not found inside tar.gz archive", targetBase)
}

func extractBinaryFromZip(archiveBytes []byte, targetBase string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "invalid zip archive: %v", err)
	}
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() && filepath.Base(f.Name) == targetBase {
			rc, oErr := f.Open()
			if oErr != nil {
				return nil, types.NewNomosError(types.ErrCodeValidationFailed, "failed reading %s from zip: %v", targetBase, oErr)
			}
			data, rErr := readBoundedEntry(rc, targetBase)
			if cErr := rc.Close(); cErr != nil {
				// closed zip entry
			}
			return data, rErr
		}
	}
	return nil, types.NewNomosError(types.ErrCodeValidationFailed, "binary %q not found inside zip archive", targetBase)
}

func readBoundedEntry(r io.Reader, name string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBinaryExtractSize+1))
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "failed extracting %s: %v", name, err)
	}
	if int64(len(data)) > maxBinaryExtractSize || len(data) == 0 {
		return nil, types.NewNomosError(types.ErrCodeValidationFailed, "extracted binary %s has invalid byte length (%d)", name, len(data))
	}
	return data, nil
}

// AtomicReplaceBinary stages binBytes into a sibling temporary file and renames it atomically into targetPath.
// @intent task_lifecycle_cli_spec.md #UpdateReport
func AtomicReplaceBinary(targetPath string, binBytes []byte) error {
	return AtomicReplaceBinaryWithBackup(targetPath, binBytes, runtime.GOOS == "windows")
}

// AtomicReplaceBinaryWithBackup stages binBytes and optionally moves an active targetPath aside (.old) first.
func AtomicReplaceBinaryWithBackup(targetPath string, binBytes []byte, stageOldAside bool) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return types.NewNomosError(types.ErrCodeBoundary, "failed preparing binary directory %s: %v", dir, err)
	}
	tmpPath, err := stageExecutableTempFile(dir, binBytes)
	if err != nil {
		return err
	}
	backupPath := ""
	if stageOldAside {
		if _, stErr := os.Stat(targetPath); stErr == nil {
			backupPath = targetPath + ".old"
			removeTempBestEffort(backupPath)
			if bErr := os.Rename(targetPath, backupPath); bErr != nil {
				removeTempBestEffort(tmpPath)
				return types.NewNomosError(types.ErrCodeSubstrate, "failed staging active binary %s aside: %v", targetPath, bErr)
			}
		}
	}
	if rErr := os.Rename(tmpPath, targetPath); rErr != nil {
		removeTempBestEffort(tmpPath)
		if backupPath != "" {
			if rbErr := os.Rename(backupPath, targetPath); rbErr != nil {
				// best-effort rollback restore
			}
		}
		return types.NewNomosError(types.ErrCodeSubstrate, "failed atomically replacing %s: %v", targetPath, rErr)
	}
	return nil
}

func stageExecutableTempFile(dir string, binBytes []byte) (string, error) {
	tmpFile, err := os.CreateTemp(dir, ".nomos-update-*")
	if err != nil {
		return "", types.NewNomosError(types.ErrCodeBoundary, "cannot stage update in %s (check write permissions): %v", dir, err)
	}
	tmpPath := tmpFile.Name()
	if err := writeSyncAndCloseTemp(tmpFile, binBytes); err != nil {
		removeTempBestEffort(tmpPath)
		return "", err
	}
	if mErr := os.Chmod(tmpPath, 0755); mErr != nil {
		removeTempBestEffort(tmpPath)
		return "", types.NewNomosError(types.ErrCodeSubstrate, "failed setting executable mode on staged binary: %v", mErr)
	}
	return tmpPath, nil
}

func writeSyncAndCloseTemp(f *os.File, data []byte) error {
	_, wErr := f.Write(data)
	sErr := f.Sync()
	cErr := f.Close()
	if wErr != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed writing staged binary: %v", wErr)
	}
	if sErr != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed syncing staged binary: %v", sErr)
	}
	if cErr != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed closing staged binary: %v", cErr)
	}
	return nil
}

func removeTempBestEffort(path string) {
	if rErr := os.Remove(path); rErr != nil && !os.IsNotExist(rErr) {
		// best-effort temp cleanup
	}
}
