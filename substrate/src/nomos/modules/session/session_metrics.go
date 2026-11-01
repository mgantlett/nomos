package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
)

func readHeadLines(path string, maxBytes int64) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(io.LimitReader(f, maxBytes))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func supplementHeadMetrics(lines []string, m *transcriptMetrics) {
	for _, line := range lines {
		if strings.Contains(line, `"type":"USER_INPUT"`) {
			m.turns++
			if m.summary == "" {
				m.summary = extractUserSummary(line)
			}
		}
	}
}

func applyFullScanMetrics(transcriptPath string, metrics *transcriptMetrics) {
	fullTurns, fullTools, errScan := scanFullTranscriptMetrics(transcriptPath)
	if errScan == nil {
		if fullTurns > 0 {
			metrics.turns = fullTurns
		}
		if fullTools > 0 {
			metrics.toolCalls = fullTools
		}
	}
}

func scanFullTranscriptMetrics(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 64*1024)
	turns, tools := 0, 0
	turnToken := []byte(`"type":"USER_INPUT"`)
	toolToken := []byte(`"tool_calls":`)

	for {
		chunk, isPrefix, errRead := reader.ReadLine()
		if len(chunk) > 0 {
			if bytes.Contains(chunk, turnToken) {
				turns++
			}
			if bytes.Contains(chunk, toolToken) {
				tools++
			}
		}
		if errRead != nil {
			break
		}
		for isPrefix {
			cont, nextPrefix, errCont := reader.ReadLine()
			isPrefix = nextPrefix
			if len(cont) > 0 {
				if bytes.Contains(cont, turnToken) {
					turns++
				}
				if bytes.Contains(cont, toolToken) {
					tools++
				}
			}
			if errCont != nil {
				break
			}
		}
	}
	return turns, tools, nil
}

func inspectTranscriptTail(lines []string) transcriptMetrics {
	var m transcriptMetrics
	for i := len(lines) - 1; i >= 0; i-- {
		processTranscriptLine(lines[i], &m)
	}
	return m
}

func processTranscriptLine(line string, m *transcriptMetrics) {
	if strings.Contains(line, `"type":"USER_INPUT"`) {
		m.turns++
		if m.summary == "" {
			m.summary = extractUserSummary(line)
		}
		if m.taskKey == "" {
			if matches := taskKeyRegex.FindAllString(line, -1); len(matches) > 0 {
				m.taskKey = matches[len(matches)-1]
			}
		}
	}
	if strings.Contains(line, `"tool_calls":`) {
		m.toolCalls++
	}
	if m.worktree == "" {
		if matches := worktreeRegex.FindStringSubmatch(line); len(matches) > 1 {
			m.worktree = matches[1]
			if m.taskKey == "" {
				if tMatch := taskKeyRegex.FindString(m.worktree); tMatch != "" {
					m.taskKey = tMatch
				}
			}
		}
	}
}

func extractUserSummary(line string) string {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return ""
	}
	content, ok := parsed["content"].(string)
	if !ok {
		return ""
	}
	return cleanUserPromptContent(content)
}

func cleanUserPromptContent(content string) string {
	if start := strings.Index(content, "<USER_REQUEST>"); start >= 0 {
		content = content[start+len("<USER_REQUEST>"):]
		if end := strings.Index(content, "</USER_REQUEST>"); end >= 0 {
			content = content[:end]
		}
	} else if start := strings.Index(content, "# Resuming from a compaction"); start >= 0 {
		if reqIdx := strings.Index(content, "1. "); reqIdx >= 0 {
			content = content[reqIdx+3:]
		}
	}

	lines := strings.Split(content, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "<") || strings.HasPrefix(trimmed, "---") {
			continue
		}
		if len(trimmed) > 80 {
			return trimmed[:77] + "..."
		}
		return trimmed
	}
	return ""
}

func readTailLines(filePath string, maxBytes int64) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	seekOffset := calculateTailOffset(stat.Size(), maxBytes)
	if _, err := file.Seek(seekOffset, io.SeekStart); err != nil {
		return nil, err
	}

	return scanLines(file)
}

func calculateTailOffset(fileSize, maxBytes int64) int64 {
	if fileSize > maxBytes {
		return fileSize - maxBytes
	}
	return 0
}

func scanLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
