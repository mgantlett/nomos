package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestAssetType_IsValid(t *testing.T) {
	tests := []struct {
		assetType AssetType
		valid     bool
	}{
		{AssetTypeHook, true},
		{AssetTypeRule, true},
		{AssetTypeDirective, true},
		{AssetTypeSchema, true},
		{AssetTypeConfig, true},
		{AssetType("workflow"), false},
		{AssetType("unknown"), false},
		{AssetType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.assetType), func(t *testing.T) {
			if got := tt.assetType.IsValid(); got != tt.valid {
				t.Errorf("AssetType(%s).IsValid() = %v; want %v", tt.assetType, got, tt.valid)
			}
		})
	}
}

func TestEmbeddedAsset_Validate(t *testing.T) {
	validSHA := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	tests := []struct {
		name    string
		asset   *EmbeddedAsset
		wantErr bool
	}{
		{
			name: "valid asset",
			asset: &EmbeddedAsset{
				RelativePath: "nomos.schema.json",
				AssetType:    AssetTypeSchema,
				SHA256:       validSHA,
				SizeBytes:    1024,
			},
			wantErr: false,
		},
		{
			name:    "nil asset",
			asset:   nil,
			wantErr: true,
		},
		{
			name: "invalid relative path characters",
			asset: &EmbeddedAsset{
				RelativePath: "invalid path with spaces.json",
				AssetType:    AssetTypeSchema,
				SHA256:       validSHA,
				SizeBytes:    100,
			},
			wantErr: true,
		},
		{
			name: "invalid asset type",
			asset: &EmbeddedAsset{
				RelativePath: "nomos.schema.json",
				AssetType:    AssetType("invalid"),
				SHA256:       validSHA,
				SizeBytes:    100,
			},
			wantErr: true,
		},
		{
			name: "invalid sha256 length",
			asset: &EmbeddedAsset{
				RelativePath: "nomos.schema.json",
				AssetType:    AssetTypeSchema,
				SHA256:       "abc123",
				SizeBytes:    100,
			},
			wantErr: true,
		},
		{
			name: "negative size bytes",
			asset: &EmbeddedAsset{
				RelativePath: "nomos.schema.json",
				AssetType:    AssetTypeSchema,
				SHA256:       validSHA,
				SizeBytes:    -5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.asset.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParityMismatch_Validate(t *testing.T) {
	tests := []struct {
		name     string
		mismatch *ParityMismatch
		wantErr  bool
	}{
		{
			name: "valid missing mismatch",
			mismatch: &ParityMismatch{
				SourcePath: "/a/source.schema.json",
				TargetPath: "/b/target.schema.json",
				SourceHash: "abc",
				Reason:     MismatchMissing,
			},
			wantErr: false,
		},
		{
			name: "valid drift mismatch",
			mismatch: &ParityMismatch{
				SourcePath: "/a/source.schema.json",
				TargetPath: "/b/target.schema.json",
				SourceHash: "abc",
				TargetHash: "def",
				Reason:     MismatchDrift,
			},
			wantErr: false,
		},
		{
			name: "invalid reason",
			mismatch: &ParityMismatch{
				SourcePath: "/a/source.schema.json",
				TargetPath: "/b/target.schema.json",
				Reason:     ParityMismatchReason("corrupted"),
			},
			wantErr: true,
		},
		{
			name:     "nil mismatch",
			mismatch: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mismatch.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckAssetParity(t *testing.T) {
	srcDir := t.TempDir()
	tgtDir := t.TempDir()

	contentA := []byte("schema A content")
	hashA := sha256Hex(contentA)
	pathA := "schemas/a.schema.json"
	writeTestFile(t, srcDir, pathA, contentA)
	writeTestFile(t, tgtDir, pathA, contentA)

	// Missing in target
	contentB := []byte("schema B content")
	hashB := sha256Hex(contentB)
	pathB := "schemas/b.schema.json"
	writeTestFile(t, srcDir, pathB, contentB)

	// Drifting in target
	contentC := []byte("schema C source content")
	hashC := sha256Hex(contentC)
	pathC := "schemas/c.schema.json"
	writeTestFile(t, srcDir, pathC, contentC)
	writeTestFile(t, tgtDir, pathC, []byte("schema C target content drift"))

	assets := []EmbeddedAsset{
		{RelativePath: pathA, AssetType: AssetTypeSchema, SHA256: hashA, SizeBytes: int64(len(contentA))},
		{RelativePath: pathB, AssetType: AssetTypeSchema, SHA256: hashB, SizeBytes: int64(len(contentB))},
		{RelativePath: pathC, AssetType: AssetTypeSchema, SHA256: hashC, SizeBytes: int64(len(contentC))},
	}

	report, err := CheckAssetParity(srcDir, tgtDir, assets)
	if err != nil {
		t.Fatalf("CheckAssetParity failed: %v", err)
	}

	if report.Passed {
		t.Errorf("expected report.Passed to be false due to missing and drifted assets")
	}

	if len(report.Mismatches) != 2 {
		t.Fatalf("expected 2 mismatches, got %d", len(report.Mismatches))
	}

	reasons := map[ParityMismatchReason]bool{}
	for _, m := range report.Mismatches {
		reasons[m.Reason] = true
	}

	if !reasons[MismatchMissing] {
		t.Errorf("expected MismatchMissing in report")
	}
	if !reasons[MismatchDrift] {
		t.Errorf("expected MismatchDrift in report")
	}
}

func TestCheckAssetParity_ZeroAssets(t *testing.T) {
	report, err := CheckAssetParity("/tmp/src", "/tmp/tgt", nil)
	if err == nil {
		t.Fatalf("expected error on zero assets")
	}
	if report.Passed {
		t.Errorf("expected report.Passed to be false on zero assets")
	}
}

func TestDiscoverParityAssets(t *testing.T) {
	tempDir := t.TempDir()
	writeTestFile(t, tempDir, "AGENTS.md", []byte("# Nomos Protocol"))
	writeTestFile(t, tempDir, "nomos.schema.json", []byte(`{"type":"object"}`))
	writeTestFile(t, tempDir, filepath.Join(".agents", "hooks.json"), []byte(`{"session-handshake":{}}`))
	writeTestFile(t, tempDir, filepath.Join(".agents", "scripts", "session-start.sh"), []byte("#!/usr/bin/env bash\necho {}\n"))

	assets, err := DiscoverParityAssets(tempDir)
	if err != nil {
		t.Fatalf("DiscoverParityAssets failed: %v", err)
	}

	if len(assets) != 4 {
		t.Fatalf("expected 4 assets discovered, got %d", len(assets))
	}

	report, pErr := CheckBilateralParity(tempDir, tempDir)
	if pErr != nil {
		t.Fatalf("CheckBilateralParity failed: %v", pErr)
	}
	if !report.Passed || report.TotalAssets != 4 {
		t.Errorf("expected clean parity pass with 4 assets, got passed=%v, total=%d", report.Passed, report.TotalAssets)
	}
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func writeTestFile(t *testing.T, base, rel string, content []byte) {
	t.Helper()
	p := filepath.Join(base, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("failed creating dir: %v", err)
	}
	if err := os.WriteFile(p, content, 0644); err != nil {
		t.Fatalf("failed writing file: %v", err)
	}
}

