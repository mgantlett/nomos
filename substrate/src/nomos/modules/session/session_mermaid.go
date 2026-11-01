package session

import (
	"fmt"
	"path/filepath"
	"strings"
)

// GenerateMermaidVisual builds an interactive Mermaid flowchart representing all active sessions.
func GenerateMermaidVisual(sessions []AgentSession) string {
	if len(sessions) == 0 {
		return emptyMermaidDiagram()
	}

	var sb strings.Builder
	writeMermaidHeader(&sb)
	writeSessionSubgraph(&sb, sessions)
	writeWorktreeConnections(&sb, sessions)
	return sb.String()
}

func emptyMermaidDiagram() string {
	return "flowchart TD\n    EMPTY[\"No active Nomos sessions detected\"]:::idle\n    classDef idle fill:#1f2937,stroke:#4b5563,stroke-width:1px,color:#9ca3af;"
}

func writeMermaidHeader(sb *strings.Builder) {
	sb.WriteString("flowchart TD\n")
	sb.WriteString("    classDef active fill:#1e40af,stroke:#3b82f6,stroke-width:2px,color:#eff6ff;\n")
	sb.WriteString("    classDef idle fill:#065f46,stroke:#10b981,stroke-width:2px,color:#ecfdf5;\n")
	sb.WriteString("    classDef completed fill:#1f2937,stroke:#4b5563,stroke-width:1px,color:#9ca3af;\n\n")
}

func writeSessionSubgraph(sb *strings.Builder, sessions []AgentSession) {
	sb.WriteString("    subgraph SESSIONS[\"Nomos Tier 1 Agent Sessions\"]\n")
	limit := len(sessions)
	if limit > 8 {
		limit = 8
	}
	for i := 0; i < limit; i++ {
		writeSessionNode(sb, sessions[i])
	}
	sb.WriteString("    end\n\n")
}

func writeSessionNode(sb *strings.Builder, s AgentSession) {
	shortID := truncateID(s.ID, 8)
	nodeID := fmt.Sprintf("SES_%s", strings.ReplaceAll(shortID, "-", ""))
	class := sessionClass(s.Status)
	taskLabel := formatTaskLabel(s.TaskKey, s.Phase)
	summary := formatSummary(s.RecentSummary)
	title := fmt.Sprintf("Session %s", shortID)
	if s.Name != "" {
		title = s.Name
	}

	sb.WriteString(fmt.Sprintf("        %s[\"<b>%s</b><br>%s<br>%s%s\"]:::%s\n",
		nodeID, title, s.AgentType, taskLabel, summary, class))
}

func sessionClass(status string) string {
	switch status {
	case "ACTIVE":
		return "active"
	case "IDLE":
		return "idle"
	default:
		return "completed"
	}
}

func formatTaskLabel(taskKey, phase string) string {
	if taskKey != "" {
		return fmt.Sprintf("Task: %s (%s)", taskKey, phase)
	}
	return "No active task"
}

func formatSummary(summary string) string {
	if summary == "" {
		return ""
	}
	escaped := strings.ReplaceAll(summary, "\"", "'")
	return fmt.Sprintf("<br><i>%s</i>", escaped)
}

func writeWorktreeConnections(sb *strings.Builder, sessions []AgentSession) {
	limit := len(sessions)
	if limit > 8 {
		limit = 8
	}
	for i := 0; i < limit; i++ {
		s := sessions[i]
		if s.WorktreePath != "" && s.TaskKey != "" {
			writeSingleConnection(sb, s)
		}
	}
}

func writeSingleConnection(sb *strings.Builder, s AgentSession) {
	shortID := truncateID(s.ID, 8)
	nodeID := fmt.Sprintf("SES_%s", strings.ReplaceAll(shortID, "-", ""))
	wtNodeID := fmt.Sprintf("WT_%s", strings.ReplaceAll(s.TaskKey, "-", "_"))
	wtBase := filepath.Base(s.WorktreePath)

	sb.WriteString(fmt.Sprintf("    %s -->|\"Worktree: %s\"| %s[\"Worktree: %s<br>Phase: %s\"]\n",
		nodeID, wtBase, wtNodeID, wtBase, s.Phase))
}

func truncateID(id string, maxLen int) string {
	if len(id) > maxLen {
		return id[:maxLen]
	}
	return id
}

// GenerateSessionThreadsMermaid constructs a causal directed graph from an individual session's events.
func GenerateSessionThreadsMermaid(detail *SessionDetail) string {
	if detail == nil {
		return "graph TD\n    EMPTY[\"No session details available\"]:::turn\n    classDef turn fill:#1e293b,stroke:#475569,stroke-width:1px,color:#f8fafc;"
	}

	var sb strings.Builder
	sb.WriteString("graph TD\n")
	sb.WriteString("    classDef turn fill:#1e293b,stroke:#475569,stroke-width:1px,color:#f8fafc;\n")
	sb.WriteString("    classDef release fill:#065f46,stroke:#10b981,stroke-width:2px,color:#ecfdf5;\n")
	sb.WriteString("    classDef active fill:#1e40af,stroke:#3b82f6,stroke-width:2px,color:#eff6ff;\n")
	sb.WriteString("    classDef error fill:#881337,stroke:#f43f5e,stroke-width:2px,color:#ffe4e6;\n\n")

	shortID := truncateID(detail.ID, 8)
	rootTitle := fmt.Sprintf("💬 Session %s (%s)", shortID, detail.AgentType)
	if detail.TaskKey != "" {
		rootTitle += fmt.Sprintf("<br>Task: %s · Phase: %s", detail.TaskKey, detail.Phase)
	}
	sb.WriteString(fmt.Sprintf("    Root[\"%s\"]:::active\n", rootTitle))

	prevNode := "Root"
	eventCount := len(detail.Events)
	if eventCount == 0 {
		summary := detail.RecentSummary
		if summary == "" {
			summary = "Live conversation arc in progress"
		}
		sb.WriteString(fmt.Sprintf("    Root --> E0[\"Turns: %d · Tools: %d<br>%s\"]:::turn\n",
			detail.TurnCount, detail.ToolCallCount, escapeMermaid(summary)))
		return sb.String()
	}

	maxEvents := 20
	startIndex := 0
	if eventCount > maxEvents {
		startIndex = eventCount - maxEvents
		sb.WriteString(fmt.Sprintf("    Root --> Skip[\"Skipped %d earlier events...\"]:::turn\n", startIndex))
		prevNode = "Skip"
	}

	for i := startIndex; i < eventCount; i++ {
		ev := detail.Events[i]
		nodeID := fmt.Sprintf("E%d", i)
		nodeClass := "turn"
		if ev.IsError {
			nodeClass = "error"
		} else if ev.Type == "RECEIPT" {
			nodeClass = "release"
		} else if ev.Type == "USER_REQUEST" {
			nodeClass = "active"
		}

		label := formatEventLabel(ev)
		sb.WriteString(fmt.Sprintf("    %s --> %s[\"%s\"]:::%s\n", prevNode, nodeID, label, nodeClass))
		prevNode = nodeID
	}

	return sb.String()
}

func formatEventLabel(ev SessionEvent) string {
	title := escapeMermaid(ev.Title)
	if ev.Tool != "" {
		return fmt.Sprintf("🔧 %s<br><i>%s</i>", ev.Tool, title)
	}
	if ev.Type == "USER_REQUEST" {
		return fmt.Sprintf("👤 %s", title)
	}
	if ev.Type == "RECEIPT" {
		return fmt.Sprintf("🚢 %s", title)
	}
	return title
}

func escapeMermaid(s string) string {
	r := strings.ReplaceAll(s, "\"", "'")
	r = strings.ReplaceAll(r, "<", "&lt;")
	r = strings.ReplaceAll(r, ">", "&gt;")
	r = strings.ReplaceAll(r, "\n", " ")
	if len(r) > 60 {
		return r[:57] + "..."
	}
	return r
}
