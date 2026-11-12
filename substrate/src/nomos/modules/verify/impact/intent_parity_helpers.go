// Package impact implements the deterministic Definition of Done (DoD) quality gate pipeline.
package impact

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ProtocolStartMarker delimits the start of the canonical Nomos protocol region (#ProtocolMarkerBounds).
	ProtocolStartMarker = "<!-- NOMOS_OPEN_CORE_PROTOCOL_START -->"
	// ProtocolEndMarker delimits the end of the canonical Nomos protocol region (#ProtocolMarkerBounds).
	ProtocolEndMarker = "<!-- NOMOS_OPEN_CORE_PROTOCOL_END -->"
	// ClaudeAgentsImport is the canonical SSoT import line required in CLAUDE.md (Law 4 & Law 6).
	ClaudeAgentsImport = "@AGENTS.md"
)

// ExtractProtocolMarkerBlock extracts the canonical marker-bounded protocol block from AGENTS.md.
// @intent INT-841 #IntentParityGate
func ExtractProtocolMarkerBlock(content []byte) ([]byte, bool) {
	text := string(content)
	startIdx := strings.Index(text, ProtocolStartMarker)
	endIdx := strings.Index(text, ProtocolEndMarker)
	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		return nil, false
	}
	block := strings.TrimSpace(text[startIdx : endIdx+len(ProtocolEndMarker)])
	return []byte(block), true
}

// ComputeMarkerOrFileHash executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func ComputeMarkerOrFileHash(path string) (string, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	if block, ok := ExtractProtocolMarkerBlock(raw); ok {
		sum := sha256.Sum256(bytes.TrimSpace(block))
		return hex.EncodeToString(sum[:]), true, nil
	}
	sum := sha256.Sum256(bytes.TrimSpace(raw))
	return hex.EncodeToString(sum[:]), false, nil
}

// AuditTemplateByteParity executes deterministic impact verification logic (@intent nomos_intent_traceability_spec.md).
func AuditTemplateByteParity(root string) []string {
	var drifts []string
	tplAgents := filepath.Join(root, "substrate", "cli", "templates", "AGENTS.md")
	drifts = append(drifts, auditProtocolDocParity(root, "AGENTS.md", tplAgents)...)
	drifts = append(drifts, auditClaudeImportParity(root, tplAgents)...)

	agentsDir := filepath.Join(root, ".agents")
	drifts = append(drifts, auditLegacyWorkflowPlaybooks(root, agentsDir)...)
	drifts = append(drifts, auditHookTemplatesParity(root, agentsDir)...)
	return drifts
}

func auditProtocolDocParity(root, docName, tplPath string) []string {
	docPath := filepath.Join(root, docName)
	fiDoc, err := os.Stat(docPath)
	if err != nil || fiDoc.IsDir() {
		return nil
	}
	docHash, hasMarkers, dErr := ComputeMarkerOrFileHash(docPath)
	if dErr != nil {
		return []string{fmt.Sprintf("failed hashing %s: %v", docName, dErr)}
	}

	fiTpl, tplErr := os.Stat(tplPath)
	if tplErr != nil || fiTpl.IsDir() {
		return nil
	}
	if !hasMarkers {
		return []string{fmt.Sprintf("%s missing canonical protocol markers (%s .. %s)", docName, ProtocolStartMarker, ProtocolEndMarker)}
	}
	tplHash, _, tErr := ComputeMarkerOrFileHash(tplPath)
	if tErr == nil && docHash != tplHash {
		return []string{fmt.Sprintf("%s protocol block drifted from template (doc=%s, tpl=%s)", docName, docHash[:8], tplHash[:8])}
	}
	return nil
}

func auditClaudeImportParity(root, tplPath string) []string {
	fiTpl, tplErr := os.Stat(tplPath)
	if tplErr != nil || fiTpl.IsDir() {
		return nil
	}
	claudePath := filepath.Join(root, "CLAUDE.md")
	fiDoc, err := os.Stat(claudePath)
	if err != nil || fiDoc.IsDir() {
		return nil
	}
	raw, rErr := os.ReadFile(claudePath)
	if rErr != nil {
		return []string{fmt.Sprintf("failed reading CLAUDE.md: %v", rErr)}
	}
	text := string(raw)
	var drifts []string
	if strings.Contains(text, ProtocolStartMarker) || strings.Contains(text, ProtocolEndMarker) {
		drifts = append(drifts, "CLAUDE.md duplicates managed protocol block; collapse to '@AGENTS.md' import (Law 4 & Law 6)")
	}
	if !containsTrimmedLine(text, ClaudeAgentsImport) {
		drifts = append(drifts, "CLAUDE.md missing canonical '@AGENTS.md' SSoT import pointer")
	}
	return drifts
}

func containsTrimmedLine(content, target string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}

func auditLegacyWorkflowPlaybooks(root, agentsDir string) []string {
	legacyWfDir := filepath.Join(agentsDir, "workflows")
	fi, err := os.Stat(legacyWfDir)
	if err != nil || !fi.IsDir() {
		return nil
	}
	tplRoot := filepath.Join(root, "substrate", "cli", "templates")
	if tfi, tErr := os.Stat(tplRoot); tErr == nil && tfi.IsDir() {
		return []string{"deprecated workflow directory detected under .agents; workflows are excised in Nomos open-core (Law 6)"}
	}
	var drifts []string
	for _, legacy := range []string{"nomos-handshake.md", "nomos-idle.md", "nomos-review.md"} {
		if _, sErr := os.Stat(filepath.Join(legacyWfDir, legacy)); sErr == nil {
			drifts = append(drifts, fmt.Sprintf("deprecated Nomos workflow playbook %s detected; run 'nomos init' to prune legacy playbooks (Law 6)", legacy))
		}
	}
	return drifts
}

func auditHookTemplatesParity(root, agentsDir string) []string {
	var drifts []string
	tplHooksDir := filepath.Join(root, "substrate", "cli", "templates", "hooks")
	if fi, err := os.Stat(tplHooksDir); err != nil || !fi.IsDir() {
		return nil
	}

	wsManifest := filepath.Join(agentsDir, "hooks.json")
	tplManifest := filepath.Join(tplHooksDir, "hooks.json")
	if d := compareAssetParity("hooks.json", wsManifest, tplManifest); d != "" {
		drifts = append(drifts, d)
	}

	tplScriptsDir := filepath.Join(tplHooksDir, "scripts")
	entries, err := os.ReadDir(tplScriptsDir)
	if err != nil {
		return drifts
	}
	wsScriptsDir := filepath.Join(agentsDir, "scripts")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		src := filepath.Join(wsScriptsDir, e.Name())
		tpl := filepath.Join(tplScriptsDir, e.Name())
		if d := compareAssetParity("hook script "+e.Name(), src, tpl); d != "" {
			drifts = append(drifts, d)
		}
	}
	return drifts
}

func compareAssetParity(label, wsPath, tplPath string) string {
	h1, _, e1 := computeFileHash(wsPath)
	h2, _, e2 := computeFileHash(tplPath)
	if e2 != nil {
		return ""
	}
	if e1 != nil {
		return fmt.Sprintf("workspace %s missing for template", label)
	}
	if h1 != h2 {
		return fmt.Sprintf("%s drifted from template (ws=%s, tpl=%s)", label, h1[:8], h2[:8])
	}
	return ""
}

func computeFileHash(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}
