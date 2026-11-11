package constants

import (
	"testing"
)

func TestConstants(t *testing.T) {
	if AppName != "nomos" {
		t.Errorf("expected AppName to be 'nomos', got %s", AppName)
	}
	if DirNomosHome != ".nomos" {
		t.Errorf("expected DirNomosHome to be '.nomos', got %s", DirNomosHome)
	}
	if DirAgents != ".agents" {
		t.Errorf("expected DirAgents to be '.agents', got %s", DirAgents)
	}
	if FileNomosYAML != "nomos.yaml" {
		t.Errorf("expected FileNomosYAML to be 'nomos.yaml', got %s", FileNomosYAML)
	}
	if FileNomosGlobalDB != "nomos_global.db" {
		t.Errorf("expected FileNomosGlobalDB to be 'nomos_global.db', got %s", FileNomosGlobalDB)
	}
	if FileMnemeDB != "mneme.db" {
		t.Errorf("expected FileMnemeDB to be 'mneme.db', got %s", FileMnemeDB)
	}
	if EnvNomosHome != "NOMOS_HOME" {
		t.Errorf("expected EnvNomosHome to be 'NOMOS_HOME', got %s", EnvNomosHome)
	}
	if EnvNomosDB != "NOMOS_DB" {
		t.Errorf("expected EnvNomosDB to be 'NOMOS_DB', got %s", EnvNomosDB)
	}
	if EnvNomosBrainDB != "NOMOS_MNEME_DB" {
		t.Errorf("expected EnvNomosBrainDB to be 'NOMOS_MNEME_DB', got %s", EnvNomosBrainDB)
	}
	if EnvNomosConfig != "NOMOS_CONFIG" {
		t.Errorf("expected EnvNomosConfig to be 'NOMOS_CONFIG', got %s", EnvNomosConfig)
	}
	if FileUpdateCheckCache != "last_update_check.json" {
		t.Errorf("expected FileUpdateCheckCache to be 'last_update_check.json', got %s", FileUpdateCheckCache)
	}
	if EnvNomosNoUpdateCheck != "NOMOS_NO_UPDATE_CHECK" {
		t.Errorf("expected EnvNomosNoUpdateCheck to be 'NOMOS_NO_UPDATE_CHECK', got %s", EnvNomosNoUpdateCheck)
	}
}
