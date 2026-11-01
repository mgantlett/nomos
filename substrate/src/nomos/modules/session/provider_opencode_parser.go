package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type openCodeSessionDoc struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	ProjectID string `json:"projectID"`
	Directory string `json:"directory"`
	Title     string `json:"title"`
	Time      struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
}

type openCodeMessageDoc struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	Role      string `json:"role"`
	Time      struct {
		Created int64 `json:"created"`
	} `json:"time"`
}

type openCodePartDoc struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionID"`
	MessageID string         `json:"messageID"`
	Type      string         `json:"type"` // "text", "tool_call", "tool_result"
	Text      string         `json:"text"`
	Call      map[string]any `json:"call,omitempty"`
}

func (p *ProviderOpenCode) populateOpenCodeDetailEvents(root, sessionID string, detail *SessionDetail, repoRoot string) {
	msgDir := filepath.Join(root, "message", sessionID)
	entries, err := os.ReadDir(msgDir)
	if err != nil {
		return
	}

	var rawLinesForSentinel []string

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, rErr := os.ReadFile(filepath.Join(msgDir, e.Name()))
		if rErr != nil {
			continue
		}
		var msg openCodeMessageDoc
		if json.Unmarshal(data, &msg) != nil {
			continue
		}

		sentinelLines := p.processOpenCodeMessageParts(root, msg, detail)
		rawLinesForSentinel = append(rawLinesForSentinel, sentinelLines...)
	}

	trimDetailHistory(detail)
	detail.Sentinel = EvaluateSentinel(detail, rawLinesForSentinel, repoRoot)
	detail.Trajectory = InferTrajectory(detail, rawLinesForSentinel)
}

func (p *ProviderOpenCode) processOpenCodeMessageParts(root string, msg openCodeMessageDoc, detail *SessionDetail) []string {
	partDir := filepath.Join(root, "part", msg.ID)
	partEntries, pErr := os.ReadDir(partDir)
	if pErr != nil {
		return nil
	}

	ts := time.UnixMilli(msg.Time.Created).Format(time.RFC3339)
	var sentinelLines []string

	for _, pe := range partEntries {
		if pe.IsDir() || !strings.HasSuffix(pe.Name(), ".json") {
			continue
		}
		pData, peErr := os.ReadFile(filepath.Join(partDir, pe.Name()))
		if peErr != nil {
			continue
		}
		var part openCodePartDoc
		if json.Unmarshal(pData, &part) != nil {
			continue
		}

		if line := p.processSinglePartRecord(msg.Role, ts, part, detail); line != "" {
			sentinelLines = append(sentinelLines, line)
		}
	}
	return sentinelLines
}

func (p *ProviderOpenCode) processSinglePartRecord(role, ts string, part openCodePartDoc, detail *SessionDetail) string {
	if role == "user" && part.Type == "text" && part.Text != "" {
		detail.RecentPrompts = append(detail.RecentPrompts, cleanSummary(part.Text))
		detail.Events = append(detail.Events, SessionEvent{
			Type:      "USER_REQUEST",
			Timestamp: ts,
			Title:     cleanSummary(part.Text),
			Detail:    cleanFullPrompt(part.Text),
		})
		return ""
	}

	if part.Type == "tool_call" || len(part.Call) > 0 {
		toolName := extractOpenCodeToolName(part.Call)
		detail.ToolsSummary[toolName]++

		detail.Events = append(detail.Events, SessionEvent{
			Type:      "TOOL_CALL",
			Timestamp: ts,
			Title:     fmt.Sprintf("Execute %s", toolName),
			Detail:    fmt.Sprintf("%v", part.Call),
			Tool:      toolName,
		})

		mockTC := rawToolCallItem{
			Name: toolName,
			Args: part.Call,
		}
		marshaled, mErr := json.Marshal(map[string]any{
			"type":       "assistant",
			"created_at": ts,
			"tool_calls": []rawToolCallItem{mockTC},
		})
		if mErr != nil {
			return ""
		}
		return string(marshaled)
	}
	return ""
}

func extractOpenCodeToolName(call map[string]any) string {
	if tn, ok := call["tool"].(string); ok && tn != "" {
		return tn
	}
	if tn, ok := call["name"].(string); ok && tn != "" {
		return tn
	}
	return "tool"
}

func (p *ProviderOpenCode) scanSessionMessages(root, sessionID string) (int, int, string) {
	msgDir := filepath.Join(root, "message", sessionID)
	entries, err := os.ReadDir(msgDir)
	if err != nil {
		return 0, 0, ""
	}

	turns := 0
	toolCalls := 0
	firstPrompt := ""

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, rErr := os.ReadFile(filepath.Join(msgDir, e.Name()))
		if rErr != nil {
			continue
		}
		var msg openCodeMessageDoc
		if json.Unmarshal(data, &msg) != nil {
			continue
		}

		if msg.Role == "user" {
			turns++
			if firstPrompt == "" {
				firstPrompt = p.readFirstMessagePrompt(root, msg.ID)
			}
		} else if msg.Role == "assistant" {
			toolCalls += p.countMessageToolCalls(root, msg.ID)
		}
	}

	return turns, toolCalls, firstPrompt
}

func (p *ProviderOpenCode) readFirstMessagePrompt(root, msgID string) string {
	partDir := filepath.Join(root, "part", msgID)
	parts, err := os.ReadDir(partDir)
	if err != nil {
		return ""
	}
	for _, pe := range parts {
		if !strings.HasSuffix(pe.Name(), ".json") {
			continue
		}
		pData, rErr := os.ReadFile(filepath.Join(partDir, pe.Name()))
		if rErr != nil {
			continue
		}
		var part openCodePartDoc
		if json.Unmarshal(pData, &part) == nil && part.Text != "" {
			return part.Text
		}
	}
	return ""
}

func (p *ProviderOpenCode) countMessageToolCalls(root, msgID string) int {
	partDir := filepath.Join(root, "part", msgID)
	parts, err := os.ReadDir(partDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, pe := range parts {
		if !strings.HasSuffix(pe.Name(), ".json") {
			continue
		}
		pData, rErr := os.ReadFile(filepath.Join(partDir, pe.Name()))
		if rErr != nil {
			continue
		}
		var part openCodePartDoc
		if json.Unmarshal(pData, &part) == nil && (part.Type == "tool_call" || len(part.Call) > 0) {
			count++
		}
	}
	return count
}
