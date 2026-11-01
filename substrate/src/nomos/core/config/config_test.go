package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/constants"
)

func TestDefaultNomosConfig(t *testing.T) {
	cfg := DefaultNomosConfig()
	if cfg == nil {
		t.Fatalf("expected non-nil default config")
	}
	if cfg.Project != constants.AppName {
		t.Errorf("expected default project %s, got %s", constants.AppName, cfg.Project)
	}
	if !cfg.Governance.Boundary.Enforce {
		t.Errorf("expected boundary enforcement true by default")
	}
	if cfg.Governance.QualityGates.Thresholds.MaxFileLOC != 400 {
		t.Errorf("expected max file loc 400, got %d", cfg.Governance.QualityGates.Thresholds.MaxFileLOC)
	}
	if len(cfg.Governance.Boundary.ProhibitedCommands) != 0 {
		t.Errorf("expected zero prohibited_commands in downstream default config, got %v", cfg.Governance.Boundary.ProhibitedCommands)
	}
	if len(cfg.Governance.Boundary.ProhibitedPackages) != 0 {
		t.Errorf("expected zero prohibited_packages in downstream default config, got %v", cfg.Governance.Boundary.ProhibitedPackages)
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
}

func TestForbiddenLiteralsOverrideDetection(t *testing.T) {
	tempDir := t.TempDir()
	emptyOverride := filepath.Join(tempDir, "empty.yaml")
	_ = os.WriteFile(emptyOverride, []byte("project: demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    forbidden_literals: []\n"), 0644)
	cfgEmpty, err := LoadConfig(emptyOverride)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if !cfgEmpty.Governance.QualityGates.HasForbiddenLiteralsOverride || len(cfgEmpty.Governance.QualityGates.ForbiddenLiterals) != 0 {
		t.Fatalf("expected explicit empty forbidden_literals override, got override=%v list=%v",
			cfgEmpty.Governance.QualityGates.HasForbiddenLiteralsOverride, cfgEmpty.Governance.QualityGates.ForbiddenLiterals)
	}

	commentedOnly := filepath.Join(tempDir, "commented.yaml")
	_ = os.WriteFile(commentedOnly, []byte("project: demo\ndefault_branch: main\ngovernance:\n  quality_gates:\n    # forbidden_literals: []\n    strict_mode: true\n"), 0644)
	cfgCommented, err := LoadConfig(commentedOnly)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfgCommented.Governance.QualityGates.HasForbiddenLiteralsOverride {
		t.Fatalf("commented-out forbidden_literals should not mark HasForbiddenLiteralsOverride=true")
	}
}

func TestLoadConfigFromDisk(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, constants.FileNomosYAML)
	sampleYAML := `
project: nomos-v4
default_branch: main
governance:
    boundary:
        enforce: true
    quality_gates:
        strict_mode: true
        thresholds:
            max_file_loc: 400
            max_func_loc: 60
            min_test_coverage_pct: 55
`
	if err := os.WriteFile(configPath, []byte(sampleYAML), 0644); err != nil {
		t.Fatalf("failed writing test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig returned unexpected error: %v", err)
	}
	if cfg.Project != "nomos-v4" {
		t.Errorf("expected project nomos-v4, got %s", cfg.Project)
	}
	if !cfg.Governance.Boundary.Enforce {
		t.Errorf("expected boundary.enforce true")
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Errorf("loaded config should validate: %v", err)
	}

	// Test LoadConfig with empty path resolving via NOMOS_CONFIG
	t.Setenv(constants.EnvNomosConfig, configPath)
	cfgAuto, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig(\"\") with env failed: %v", err)
	}
	if cfgAuto.Project != "nomos-v4" {
		t.Errorf("expected project nomos-v4, got %s", cfgAuto.Project)
	}
}

func TestLoadConfigDefaultFallback(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv(constants.EnvNomosConfig, filepath.Join(tempHome, "nonexistent.yaml"))

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("expected fallback to default config, got error: %v", err)
	}
	if cfg.Project != constants.AppName {
		t.Errorf("expected fallback project %s, got %s", constants.AppName, cfg.Project)
	}
}

func TestLoadConfigInvalidPath(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/to/nomos.yaml")
	if err == nil {
		t.Errorf("expected error for nonexistent config path")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, constants.FileNomosYAML)
	if err := os.WriteFile(configPath, []byte("invalid: yaml: :"), 0644); err != nil {
		t.Fatalf("failed writing test file: %v", err)
	}
	_, err := LoadConfig(configPath)
	if err == nil {
		t.Errorf("expected error for corrupted YAML")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*EcosystemConfig)
		wantErr bool
	}{
		{
			name:    "nil config",
			mutate:  nil,
			wantErr: true,
		},
		{
			name: "empty project",
			mutate: func(c *EcosystemConfig) {
				c.Project = ""
			},
			wantErr: true,
		},
		{
			name: "empty default branch",
			mutate: func(c *EcosystemConfig) {
				c.DefaultBranch = ""
			},
			wantErr: true,
		},
		{
			name: "negative test coverage threshold",
			mutate: func(c *EcosystemConfig) {
				c.Governance.QualityGates.Thresholds.MinTestCoveragePct = -5
			},
			wantErr: true,
		},
		{
			name: "coverage threshold over 100",
			mutate: func(c *EcosystemConfig) {
				c.Governance.QualityGates.Thresholds.MinTestCoveragePct = 105
			},
			wantErr: true,
		},
		{
			name: "zero max file loc",
			mutate: func(c *EcosystemConfig) {
				c.Governance.QualityGates.Thresholds.MaxFileLOC = 0
			},
			wantErr: true,
		},
		{
			name: "zero max func loc",
			mutate: func(c *EcosystemConfig) {
				c.Governance.QualityGates.Thresholds.MaxFuncLOC = 0
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mutate == nil {
				if err := ValidateConfig(nil); (err != nil) != tt.wantErr {
					t.Errorf("ValidateConfig(nil) error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			cfg := DefaultNomosConfig()
			tt.mutate(cfg)
			err := ValidateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathDiscovery(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(constants.EnvNomosHome, tempDir)

	if got := GetNomosHome(); got != tempDir {
		t.Errorf("expected nomos home %s, got %s", tempDir, got)
	}

	expectedDB := filepath.Join(tempDir, constants.DirData, constants.DirDB, constants.FileNomosGlobalDB)
	if got := ResolveGlobalDBPath(); got != expectedDB {
		t.Errorf("expected global db path %s, got %s", expectedDB, got)
	}

	expectedBrainDB := filepath.Join(tempDir, constants.DirData, constants.DirDB, constants.FileMnemeDB)
	if got := ResolveMnemeDBPath(); got != expectedBrainDB {
		t.Errorf("expected brain db path %s, got %s", expectedBrainDB, got)
	}
}

func TestPathDiscoveryEnvOverrides(t *testing.T) {
	customDB := "/tmp/custom_nomos.db"
	customBrainDB := "/tmp/custom_brain.db"
	t.Setenv(constants.EnvNomosDB, customDB)
	t.Setenv(constants.EnvNomosBrainDB, customBrainDB)

	if got := ResolveGlobalDBPath(); got != customDB {
		t.Errorf("expected global db override %s, got %s", customDB, got)
	}
	if got := ResolveMnemeDBPath(); got != customBrainDB {
		t.Errorf("expected brain db override %s, got %s", customBrainDB, got)
	}
}

func TestLocateConfigPath(t *testing.T) {
	tempDir := t.TempDir()
	cfgPath := filepath.Join(tempDir, constants.FileNomosYAML)
	if err := os.WriteFile(cfgPath, []byte("project: test\n"), 0644); err != nil {
		t.Fatalf("failed writing test config: %v", err)
	}

	t.Setenv(constants.EnvNomosConfig, cfgPath)
	found, err := LocateConfigPath()
	if err != nil {
		t.Fatalf("LocateConfigPath failed: %v", err)
	}
	if found != cfgPath {
		t.Errorf("expected %s, got %s", cfgPath, found)
	}
}

func TestLocateHomeConfig(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv(constants.EnvNomosConfig, "")

	homeConfigDir := filepath.Join(tempHome, constants.DirNomosHome, constants.DirConfig)
	if err := os.MkdirAll(homeConfigDir, 0755); err != nil {
		t.Fatalf("failed creating home config dir: %v", err)
	}
	cfgPath := filepath.Join(homeConfigDir, constants.FileNomosYAML)
	if err := os.WriteFile(cfgPath, []byte("project: home-test\n"), 0644); err != nil {
		t.Fatalf("failed writing home config: %v", err)
	}

	found := locateHomeConfig()
	if found != cfgPath {
		t.Errorf("expected %s, got %s", cfgPath, found)
	}
}

func TestCheckConfigPaths(t *testing.T) {
	tempDir := t.TempDir()
	agentsDir := filepath.Join(tempDir, constants.DirAgents)
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("failed creating agents dir: %v", err)
	}
	cfgPath := filepath.Join(agentsDir, constants.FileNomosYAML)
	if err := os.WriteFile(cfgPath, []byte("project: agent-test\n"), 0644); err != nil {
		t.Fatalf("failed writing agent config: %v", err)
	}

	found := checkConfigPaths(tempDir)
	if found != cfgPath {
		t.Errorf("expected %s, got %s", cfgPath, found)
	}

	emptyDir := t.TempDir()
	if got := checkConfigPaths(emptyDir); got != "" {
		t.Errorf("expected empty string for dir without config, got %s", got)
	}
}

func TestResolveIntentPath(t *testing.T) {
	tempDir := t.TempDir()
	intentDir := filepath.Join(tempDir, constants.DirIntent)
	if err := os.MkdirAll(intentDir, 0755); err != nil {
		t.Fatalf("failed making intent dir: %v", err)
	}

	subDir := filepath.Join(tempDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed making subDir: %v", err)
	}

	resolved := ResolveIntentPath(subDir)
	if resolved != intentDir {
		t.Errorf("expected resolved intent dir %s, got %s", intentDir, resolved)
	}

	// Empty string defaults to cwd search
	_ = ResolveIntentPath("")

	// Directory without intent anywhere in hierarchy returns empty string
	emptyRoot, err := os.MkdirTemp("/tmp", "nomos_no_intent_*")
	if err != nil {
		t.Fatalf("failed creating system temp dir: %v", err)
	}
	defer os.RemoveAll(emptyRoot)
	nonExistent := ResolveIntentPath(emptyRoot)
	if nonExistent != "" {
		t.Errorf("expected empty string, got %s", nonExistent)
	}
}



