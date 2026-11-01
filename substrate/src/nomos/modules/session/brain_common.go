package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// parseBrainSessionDir extracts session metrics from a Gemini brain directory and transcript log.
func parseBrainSessionDir(sessDir, id, agentType string, activeWorktrees map[string]string) (*AgentSession, error) {
	transcriptPath := filepath.Join(sessDir, ".system_generated", "logs", "transcript.jsonl")
	info, err := os.Stat(transcriptPath)
	if err != nil {
		return nil, err
	}
	tailLines, err := readTailLines(transcriptPath, 256*1024)
	if err != nil {
		return nil, err
	}

	metrics := inspectTranscriptTail(tailLines)
	if metrics.turns == 0 || metrics.summary == "" {
		if headLines, errHead := readHeadLines(transcriptPath, 128*1024); errHead == nil {
			supplementHeadMetrics(headLines, &metrics)
		}
	}
	applyFullScanMetrics(transcriptPath, &metrics)

	worktreePath, taskKey, phase := resolveSessionWorktree(metrics.taskKey, metrics.worktree, activeWorktrees)
	status, finalPhase := calculateSessionStatus(info.ModTime(), phase)

	var transcriptHead string
	if headLines, errHead := readHeadLines(transcriptPath, 64*1024); errHead == nil && len(headLines) > 0 {
		transcriptHead = headLines[0]
	}
	repo := detectSessionRepo(worktreePath, transcriptHead, "")

	return &AgentSession{
		ID:            id,
		Name:          readSessionCustomName(sessDir),
		AgentType:     agentType,
		TaskKey:       taskKey,
		WorktreePath:  worktreePath,
		Repo:          repo,
		Phase:         finalPhase,
		TurnCount:     metrics.turns,
		ToolCallCount: metrics.toolCalls,
		LastActiveAt:  info.ModTime(),
		Status:        status,
		RecentSummary: metrics.summary,
	}, nil
}

// populateBrainDetail builds deep inspection telemetry from a Gemini brain transcript.
func populateBrainDetail(detail *SessionDetail, transcriptPath, repoRoot string) error {
	lines, errRead := readTailLines(transcriptPath, 4*1024*1024)
	if errRead != nil {
		return errRead
	}
	populateDetailFromLines(detail, lines)
	detail.Sentinel = EvaluateSentinel(detail, lines, repoRoot)
	detail.Trajectory = InferTrajectory(detail, lines)
	return nil
}

// resolveBrainSessionDir matches a session directory inside a brain root.
func resolveBrainSessionDir(brainRoot, sessionID string) (string, string, error) {
	if brainRoot == "" {
		return "", "", fmt.Errorf("brain root is empty")
	}
	target := strings.ToLower(strings.TrimSpace(sessionID))
	exactPath := filepath.Join(brainRoot, target)
	if info, err := os.Stat(exactPath); err == nil && info.IsDir() {
		return exactPath, target, nil
	}

	entries, err := os.ReadDir(brainRoot)
	if err != nil {
		return "", "", err
	}

	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), target) {
			return filepath.Join(brainRoot, e.Name()), e.Name(), nil
		}
	}
	return "", "", fmt.Errorf("session %q not found in brain root %s", sessionID, brainRoot)
}

// saveBrainSessionName updates or creates session_meta.json in a Gemini brain directory.
func saveBrainSessionName(brainRoot, sessionID, name string) (*SessionMetadata, error) {
	targetDir, fullID, err := resolveBrainSessionDir(brainRoot, sessionID)
	if err != nil {
		return nil, err
	}
	metaPath := filepath.Join(targetDir, SessionMetaFileName)

	var meta SessionMetadata
	if data, errRead := os.ReadFile(metaPath); errRead == nil {
		if errUn := json.Unmarshal(data, &meta); errUn != nil {
			meta = SessionMetadata{}
		}
	}

	meta.ID = fullID
	meta.Name = name
	meta.UpdatedAt = time.Now().UTC()

	data, errMarshal := json.MarshalIndent(meta, "", "  ")
	if errMarshal != nil {
		return nil, fmt.Errorf("failed marshaling session metadata: %w", errMarshal)
	}

	if errWrite := os.WriteFile(metaPath, data, 0644); errWrite != nil {
		return nil, fmt.Errorf("failed writing session metadata: %w", errWrite)
	}

	return &meta, nil
}
