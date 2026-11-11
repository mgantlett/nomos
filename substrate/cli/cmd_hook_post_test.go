package cli

import (
	"strings"
	"testing"
)

func TestEvaluateCommandGuard_DirectSQLiteInterceptUsesLaw1(t *testing.T) {
	dec := evaluateCommandGuard("sqlite3 mneme.db 'SELECT 1'", "", "")
	if dec.Decision != hookDecisionDeny {
		t.Fatalf("expected direct sqlite3 command denied, got %+v", dec)
	}
	if !strings.Contains(dec.Reason, "Law 1") || strings.Contains(dec.Reason, "ADR-0062") {
		t.Fatalf("expected Law 1 encapsulation message without private ADR-0062, got: %s", dec.Reason)
	}
}
