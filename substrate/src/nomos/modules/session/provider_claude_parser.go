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
	Type     string         `json:"type"` // "text", "thinking", "tool_use", "tool_result"
	Text     string         `json:"text,omitempty"`
	Thinking string         `json:"thinking,omitempty"`
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name,omitempty"`
	Input    map[string]any `json:"input,omitempty"`
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
	for _, key := range []string{"command", "file_path", "path", "pattern", "url", "prompt"} {
		if val, ok := input[key].(string); ok && val != "" {
			return val
		}
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
	if len(items) == 0 && text == "" {
		return ""
	}
	var toolCalls []rawToolCallItem

	for _, it := range items {
		if it.Type == "thinking" && strings.TrimSpace(it.Thinking) != "" {
			detail.Events = append(detail.Events, SessionEvent{
				Type:      "THINKING",
				Timestamp: ts,
				Title:     "Claude Deliberation",
				Detail:    it.Thinking,
			})
			continue
		}
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
		toolCalls = append(toolCalls, rawToolCallItem{
			Name: it.Name,
			Args: it.Input,
		})
	}
	if len(toolCalls) == 0 {
		return ""
	}
	marshaled, mErr := json.Marshal(map[string]any{
		"type":       "assistant",
		"created_at": ts,
		"tool_calls": toolCalls,
	})
	if mErr != nil {
		return ""
	}
	return string(marshaled)
}
