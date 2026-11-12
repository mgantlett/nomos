package selfupdate_test

import (
	"testing"

	"github.com/mgantlett/nomos/substrate/cli/selfupdate"
)

func TestNormalizeReleaseTag(t *testing.T) {
	valid := map[string]string{
		"4.14.0":        "v4.14.0",
		"v4.15.2":       "v4.15.2",
		"v5.0.0-rc.1":   "v5.0.0-rc.1",
		" 4.14.1+meta ": "v4.14.1+meta",
	}
	for in, want := range valid {
		got, err := selfupdate.NormalizeReleaseTag(in)
		if err != nil || got != want {
			t.Fatalf("NormalizeReleaseTag(%q) = (%q, %v), want (%q, nil)", in, got, err, want)
		}
	}

	invalid := []string{"", "v", "latest", "../v4.1.0", "v4.1.0/../../etc", "v4.1", "v4.1.0?foo=bar"}
	for _, bad := range invalid {
		if _, err := selfupdate.NormalizeReleaseTag(bad); err == nil {
			t.Fatalf("expected NormalizeReleaseTag(%q) to fail closed", bad)
		}
	}
}

func TestCompareSemver(t *testing.T) {
	if selfupdate.CompareSemver("4.14.0", "v4.14.0") != 0 {
		t.Fatalf("expected equal semver")
	}
	if selfupdate.CompareSemver("4.14.0", "4.15.0") >= 0 {
		t.Fatalf("expected 4.14.0 < 4.15.0")
	}
	if selfupdate.CompareSemver("4.15.0-rc.1", "4.15.0") >= 0 {
		t.Fatalf("expected prerelease < stable release")
	}
	if selfupdate.CompareSemver("5.0.0", "4.99.99") <= 0 {
		t.Fatalf("expected 5.0.0 > 4.99.99")
	}
}

func TestFormatReleaseAssetName(t *testing.T) {
	gotLinux, err := selfupdate.FormatReleaseAssetName("linux", "amd64")
	if err != nil || gotLinux != "nomos_Linux_x86_64.tar.gz" {
		t.Fatalf("got (%q, %v)", gotLinux, err)
	}
	gotDarwin, err := selfupdate.FormatReleaseAssetName("darwin", "arm64")
	if err != nil || gotDarwin != "nomos_Darwin_arm64.tar.gz" {
		t.Fatalf("got (%q, %v)", gotDarwin, err)
	}
	gotWin, err := selfupdate.FormatReleaseAssetName("windows", "amd64")
	if err != nil || gotWin != "nomos_Windows_x86_64.zip" {
		t.Fatalf("got (%q, %v)", gotWin, err)
	}
	if _, badErr := selfupdate.FormatReleaseAssetName("plan9", "amd64"); badErr == nil {
		t.Fatalf("expected unsupported OS error")
	}
}
