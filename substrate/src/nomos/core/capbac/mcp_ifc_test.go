package capbac_test

import (
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
)

func TestMCPBoundaryInterceptor_ComplianceVectors(t *testing.T) {
	t.Setenv("NOMOS_HOME", t.TempDir())
	wsDir := t.TempDir()

	// Coordinate parsing across Claude Code ('mcp__*__*') and Cursor/AGY ('mcp_*_*').
	claudeInv, ok := capbac.ParseMCPInvocation("mcp__github__merge_pull_request", "", `{"pull_number":42}`)
	if !ok || claudeInv.Server != "github" || claudeInv.Action != "merge_pull_request" || claudeInv.Mode != capbac.MCPModePrivileged {
		t.Fatalf("unexpected Claude MCP coordinate parse: %+v (ok=%v)", claudeInv, ok)
	}
	cursorInv, ok := capbac.ParseMCPInvocation("mcp_postgres_query", "", `{"sql":"SELECT 1"}`)
	if !ok || cursorInv.Server != "postgres" || cursorInv.Action != "query" || cursorInv.Mode != capbac.MCPModeRead {
		t.Fatalf("unexpected Cursor MCP coordinate parse: %+v (ok=%v)", cursorInv, ok)
	}

	// TC-MCP-02: Secret token in MCP arguments is denied.
	fakeKey := "sk-proj-" + strings.Repeat("A1b2C3", 5)
	secInv, _ := capbac.ParseMCPInvocation("mcp__slack__post_message", "", `{"text":"key=`+fakeKey+`"}`)
	secDec := capbac.EvaluateMCPBoundary(wsDir, secInv, capbac.MCPBoundaryPolicy{})
	if secDec.Allowed || !strings.Contains(secDec.Reason, "OpenAI API key") {
		t.Fatalf("TC-MCP-02: expected secret in MCP payload to be denied, got %+v", secDec)
	}

	// TC-MCP-03: Blocked MCP server policy fails closed.
	polInv, _ := capbac.ParseMCPInvocation("mcp__shadow__list_items", "", `{}`)
	polDec := capbac.EvaluateMCPBoundary(wsDir, polInv, capbac.MCPBoundaryPolicy{
		BlockedServers: []string{"shadow"},
	})
	if polDec.Allowed || !strings.Contains(polDec.Reason, "blocked_mcp_servers") {
		t.Fatalf("TC-MCP-03: expected blocked MCP server deny, got %+v", polDec)
	}
	allowListDec := capbac.EvaluateMCPBoundary(wsDir, polInv, capbac.MCPBoundaryPolicy{
		AllowedServers: []string{"github", "linear"},
	})
	if allowListDec.Allowed || !strings.Contains(allowListDec.Reason, "allowed_mcp_servers") {
		t.Fatalf("TC-MCP-03b: expected unlisted MCP server deny under allowlist, got %+v", allowListDec)
	}

	// TC-MCP-05: Cross-server untrusted ingress (fetch) -> privileged remote mutation (github) blocked until nomos verify.
	fetchInv, _ := capbac.ParseMCPInvocation("mcp__fetch__fetch", "", `{"url":"https://untrusted.example/pr-note"}`)
	if fetchDec := capbac.EvaluateMCPBoundary(wsDir, fetchInv, capbac.MCPBoundaryPolicy{}); !fetchDec.Allowed {
		t.Fatalf("TC-MCP-05: expected mcp__fetch__fetch read allow, got %+v", fetchDec)
	}

	mergeBlocked := capbac.EvaluateMCPBoundary(wsDir, claudeInv, capbac.MCPBoundaryPolicy{})
	if mergeBlocked.Allowed || mergeBlocked.ViolatedLabel != capbac.TaintMCPUntrustedIngress {
		t.Fatalf("TC-MCP-05: expected cross-server mcp__github__merge_pull_request blocked, got %+v", mergeBlocked)
	}

	// Sanitize via nomos verify -> privileged MCP call now permitted.
	san := capbac.EvaluateWorkspaceToolFlow(wsDir, capbac.ToolFlowInvocation{ToolName: "Bash", CommandLine: "nomos verify"})
	if !san.Allowed {
		t.Fatalf("expected nomos verify sanitizer allow, got %+v", san)
	}
	mergeAllowed := capbac.EvaluateMCPBoundary(wsDir, claudeInv, capbac.MCPBoundaryPolicy{})
	if !mergeAllowed.Allowed {
		t.Fatalf("TC-MCP-05: expected mcp__github__merge_pull_request allowed after nomos verify, got %+v", mergeAllowed)
	}

	// TC-MCP-04: Credential read taints session -> mutating MCP egress blocked.
	credRead := capbac.EvaluateWorkspaceToolFlow(wsDir, capbac.ToolFlowInvocation{ToolName: "Read", TargetFile: wsDir + "/.env"})
	if !credRead.Allowed {
		t.Fatalf("expected Read .env allow, got %+v", credRead)
	}
	issueInv, _ := capbac.ParseMCPInvocation("mcp__github__create_issue", "", `{"title":"debug"}`)
	issueBlocked := capbac.EvaluateMCPBoundary(wsDir, issueInv, capbac.MCPBoundaryPolicy{})
	if issueBlocked.Allowed || issueBlocked.ViolatedLabel != capbac.TaintCredentialSource {
		t.Fatalf("TC-MCP-04: expected mutating MCP call blocked under credential taint, got %+v", issueBlocked)
	}
}

