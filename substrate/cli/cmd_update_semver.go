// Package cli implements semantic version parsing, release tag validation, and archive asset naming.
package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// NormalizeReleaseTag validates a release tag and normalizes it to 'vMAJOR.MINOR.PATCH[-PRERELEASE]'.
// @intent task_lifecycle_cli_spec.md #UpdateReport
func NormalizeReleaseTag(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", types.NewNomosError(types.ErrCodeValidationFailed, "release version tag must not be empty")
	}
	ver := strings.TrimPrefix(trimmed, "v")
	if ver == "" || strings.ContainsAny(ver, "/\\?&# \t\r\n") || strings.Contains(ver, "..") {
		return "", types.NewNomosError(types.ErrCodeValidationFailed, "invalid release version tag %q", raw)
	}
	if _, _, _, _, ok := parseSemverParts(ver); !ok {
		return "", types.NewNomosError(types.ErrCodeValidationFailed, "malformed semantic version %q (expected vMAJOR.MINOR.PATCH)", raw)
	}
	return "v" + ver, nil
}

// CompareSemver compares two semantic version strings (returning -1 if a < b, 0 if a == b, 1 if a > b).
// @intent task_lifecycle_cli_spec.md #UpdateCheckCache
func CompareSemver(a, b string) int {
	majA, minA, patA, preA, okA := parseSemverParts(strings.TrimPrefix(strings.TrimSpace(a), "v"))
	majB, minB, patB, preB, okB := parseSemverParts(strings.TrimPrefix(strings.TrimSpace(b), "v"))
	if !okA || !okB {
		return strings.Compare(strings.TrimPrefix(a, "v"), strings.TrimPrefix(b, "v"))
	}
	if majA != majB {
		return cmpInt(majA, majB)
	}
	if minA != minB {
		return cmpInt(minA, minB)
	}
	if patA != patB {
		return cmpInt(patA, patB)
	}
	return comparePrerelease(preA, preB)
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	return 1
}

func comparePrerelease(preA, preB string) int {
	if preA == preB {
		return 0
	}
	if preA == "" {
		return 1
	}
	if preB == "" {
		return -1
	}
	return strings.Compare(preA, preB)
}

func parseSemverParts(ver string) (int, int, int, string, bool) {
	core, pre := splitSemverCoreAndPre(ver)
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return 0, 0, 0, "", false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	pat, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil || maj < 0 || min < 0 || pat < 0 {
		return 0, 0, 0, "", false
	}
	return maj, min, pat, pre, true
}

func splitSemverCoreAndPre(ver string) (string, string) {
	core := ver
	if idx := strings.IndexByte(ver, '+'); idx != -1 {
		core = ver[:idx]
	}
	if idx := strings.IndexByte(core, '-'); idx != -1 {
		return core[:idx], core[idx+1:]
	}
	return core, ""
}

// FormatReleaseAssetName formats the GoReleaser archive filename for a target GOOS and GOARCH.
// @intent task_lifecycle_cli_spec.md #UpdateReport
func FormatReleaseAssetName(goos, goarch string) (string, error) {
	titleOS, ext, err := mapArchiveOS(goos)
	if err != nil {
		return "", err
	}
	archLabel, aErr := mapArchiveArch(goarch)
	if aErr != nil {
		return "", aErr
	}
	return fmt.Sprintf("nomos_%s_%s.%s", titleOS, archLabel, ext), nil
}

func mapArchiveOS(goos string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(goos)) {
	case "linux":
		return "Linux", "tar.gz", nil
	case "darwin":
		return "Darwin", "tar.gz", nil
	case "windows":
		return "Windows", "zip", nil
	default:
		return "", "", types.NewNomosError(types.ErrCodeValidationFailed, "unsupported operating system %q for binary update", goos)
	}
}

func mapArchiveArch(goarch string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(goarch)) {
	case "amd64", "x86_64":
		return "x86_64", nil
	case "arm64", "aarch64":
		return "arm64", nil
	default:
		return "", types.NewNomosError(types.ErrCodeValidationFailed, "unsupported architecture %q for binary update", goarch)
	}
}

