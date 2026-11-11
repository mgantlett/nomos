package cli

import (
	"strings"
	"testing"
)

func TestHelpTaskContracts_ZeroPrivateADRCitations(t *testing.T) {
	texts := []string{helpTaskSyncLong, helpTaskProjectLong, helpTaskChoreLong, helpTaskPairLong}
	for _, txt := range texts {
		if strings.Contains(txt, "ADR-0062") || strings.Contains(txt, "ADR-0031") {
			t.Fatalf("expected zero private ADR citations in CLI help text, got:\n%s", txt)
		}
	}
}
