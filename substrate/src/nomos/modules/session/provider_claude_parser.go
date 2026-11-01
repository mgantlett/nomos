package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type claudeRecord struct {
	Type      string        `json:"type"` // "user", "assistant"
	UUID      string        `json:"uuid"`
	SessionID string        `json:"sessionId"`
	Cwd       string        `json:"cwd"`
	GitBranch string        `json:"gitBranch"`
	Timestamp string        `json:"timestamp"`
	Message   claudeMessage `json:"message"`
}

type claudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type claudeContentItem struct {
	Type  string         `json:"type"` // "text", "tool_use", "tool_result"
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

func readClaudeLines(path string) ([]claudeRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var records []claudeRecord
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		var rec claudeRecord
		if errUn := json.Unmarshal([]byte(line), &rec); errUn == nil {
			records = append(records, rec)
		}
	}
	return records, scanner.Err()
}

func parseClaudeContent(raw json.RawMessage) ([]claudeContentItem, string) {
	if len(raw) == 0 {
		return nil, ""
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return []claudeContentItem{{Type: "text", Text: str}}, str
	}

	var items []claudeContentItem
	if err := json.Unmarshal(raw, &items); err == nil {
		var texts []string
		for _, it := range items {
			if it.Type == "text" && it.Text != "" {
				texts = append(texts, it.Text)
			}
		}
		return items, strings.Join(texts, "\n")
	}

	return nil, ""
}

func extractClaudeToolDetail(input map[string]any) string {
	if cmd, ok := input["command"].(string); ok && cmd != "" {
		return cmd
	}
	if fp, ok := input["file_path"].(string); ok && fp != "" {
		return fp
	}
	return ""
}

func (p *ProviderClaudeCode) buildDetailEvents(records []claudeRecord, detail *SessionDetail, repoRoot string) []string {
	var rawLinesForSentinel []string

	for _, rec := range records {
		ts := rec.Timestamp
		if rec.Type == "user" {
			p.processClaudeUserRecord(rec, detail)
		} else if rec.Type == "assistant" {
			sentinelLine := p.processClaudeAssistantRecord(rec, detail, ts)
			if sentinelLine != "" {
				rawLinesForSentinel = append(rawLinesForSentinel, sentinelLine)
			}
		}
	}

	trimDetailHistory(detail)
	detail.Sentinel = EvaluateSentinel(detail, rawLinesForSentinel, repoRoot)
	detail.Trajectory = InferTrajectory(detail, rawLinesForSentinel)
	return rawLinesForSentinel
}

func (p *ProviderClaudeCode) processClaudeUserRecord(rec claudeRecord, detail *SessionDetail) {
	items, text := parseClaudeContent(rec.Message.Content)
	for _, it := range items {
		if it.Type == "tool_result" {
			return
		}
	}
	if text == "" {
		return
	}
	detail.RecentPrompts = append(detail.RecentPrompts, cleanSummary(text))
	detail.Events = append(detail.Events, SessionEvent{
		Type:      "USER_REQUEST",
		Timestamp: rec.Timestamp,
		Title:     cleanSummary(text),
		Detail:    cleanFullPrompt(text),
	})
}

func (p *ProviderClaudeCode) processClaudeAssistantRecord(rec claudeRecord, detail *SessionDetail, ts string) string {
	items, text := parseClaudeContent(rec.Message.Content)
	var lastSentinelLine string

	for _, it := range items {
		if it.Type != "tool_use" {
			continue
		}
		detail.ToolsSummary[it.Name]++
		detail.Events = append(detail.Events, SessionEvent{
			Type:      "TOOL_CALL",
			Timestamp: ts,
			Title:     fmt.Sprintf("Execute %s", it.Name),
			Detail:    extractClaudeToolDetail(it.Input),
			Tool:      it.Name,
		})

		mockTC := rawToolCallItem{
			Name: it.Name,
			Args: it.Input,
		}
		if marshaled, mErr := json.Marshal(map[string]any{
			"type":       "assistant",
			"created_at": ts,
			"tool_calls": []rawToolCallItem{mockTC},
		}); mErr == nil {
			lastSentinelLine = string(marshaled)
		}
	}

	if text != "" && strings.Contains(strings.ToLower(text), "think") {
		detail.Events = append(detail.Events, SessionEvent{
			Type:      "THINKING",
			Timestamp: ts,
			Title:     "Claude Deliberation",
			Detail:    text,
		})
	}

	return lastSentinelLine
}
