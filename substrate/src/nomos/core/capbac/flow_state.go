// Package capbac implements Capability-Based Access Control (CapBAC), compositional
// capability-flow taint tracking, and zero-trust boundary contracts.
package capbac

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EvaluateWorkspaceToolFlow loads the active workspace CapabilityFlowGraph, evaluates inv,
// persists updated taint edges when allowed, and returns the resulting FlowDecision.
// @intent capability_flow_taint_guard_spec.md #CapabilityFlowGraph
func EvaluateWorkspaceToolFlow(workspaceDir string, inv ToolFlowInvocation) FlowDecision {
	graph := LoadWorkspaceFlowGraph(workspaceDir)
	dec := graph.EvaluateToolFlow(inv)
	if dec.Allowed {
		if saveErr := SaveWorkspaceFlowGraph(workspaceDir, graph); saveErr != nil {
			dec.Reason = strings.TrimSpace(dec.Reason)
		}
	}
	return dec
}

// LoadWorkspaceFlowGraph reads the workspace CapabilityFlowGraph from the local hook state cache.
// @intent capability_flow_taint_guard_spec.md #CapabilityFlowGraph
func LoadWorkspaceFlowGraph(workspaceDir string) CapabilityFlowGraph {
	cleanWS := filepath.Clean(strings.TrimSpace(workspaceDir))
	base := CapabilityFlowGraph{
		Version:   1,
		Workspace: cleanWS,
	}
	statePath, err := resolveFlowStateFilePath(cleanWS)
	if err != nil || statePath == "" {
		return base
	}
	fi, statErr := os.Lstat(statePath)
	if statErr != nil || fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return base
	}
	raw, readErr := os.ReadFile(statePath)
	if readErr != nil {
		return base
	}
	var loaded CapabilityFlowGraph
	if uErr := json.Unmarshal(raw, &loaded); uErr != nil {
		return base
	}
	loaded.Version = 1
	loaded.Workspace = cleanWS
	return loaded
}

// SaveWorkspaceFlowGraph atomically writes the workspace CapabilityFlowGraph with 0600 permissions.
// @intent capability_flow_taint_guard_spec.md #CapabilityFlowGraph
func SaveWorkspaceFlowGraph(workspaceDir string, graph CapabilityFlowGraph) error {
	cleanWS := filepath.Clean(strings.TrimSpace(workspaceDir))
	statePath, err := resolveFlowStateFilePath(cleanWS)
	if err != nil || statePath == "" {
		return err
	}
	if fi, statErr := os.Lstat(statePath); statErr == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlinked flow state file: %s", statePath)
	}
	graph.Version = 1
	graph.Workspace = cleanWS
	data, mErr := json.Marshal(graph)
	if mErr != nil {
		return mErr
	}
	return os.WriteFile(statePath, data, 0600)
}

// ResetWorkspaceFlowState clears active taint edges for workspaceDir upon session initialization.
// @intent capability_flow_taint_guard_spec.md #CapabilityFlowGraph
func ResetWorkspaceFlowState(workspaceDir string) error {
	cleanWS := filepath.Clean(strings.TrimSpace(workspaceDir))
	statePath, err := resolveFlowStateFilePath(cleanWS)
	if err != nil || statePath == "" {
		return err
	}
	if remErr := os.Remove(statePath); remErr != nil && !os.IsNotExist(remErr) {
		return remErr
	}
	return nil
}

func resolveFlowStateFilePath(workspaceDir string) (string, error) {
	sum := sha256.Sum256([]byte(filepath.Clean(workspaceDir)))
	name := fmt.Sprintf("nomos-capflow-%s.json", hex.EncodeToString(sum[:8]))

	baseDir := strings.TrimSpace(os.Getenv("NOMOS_HOME"))
	if baseDir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil || cacheDir == "" {
			cacheDir = filepath.Join(os.TempDir(), fmt.Sprintf("nomos-cache-%d", os.Getuid()))
		}
		baseDir = filepath.Join(cacheDir, "nomos")
	}

	hookDir := filepath.Join(baseDir, "hooks")
	if mkErr := os.MkdirAll(hookDir, 0700); mkErr != nil {
		return "", mkErr
	}
	fi, statErr := os.Lstat(hookDir)
	if statErr != nil {
		return "", statErr
	}
	if !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("invalid capflow directory %s", hookDir)
	}
	return filepath.Join(hookDir, name), nil
}
