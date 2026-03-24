package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ClaudeStreamEvent represents a single line from Claude's stream-json output
type ClaudeStreamEvent struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`

	// Result fields
	IsError    bool `json:"is_error,omitempty"`
	DurationMs int  `json:"duration_ms,omitempty"`
	NumTurns   int  `json:"num_turns,omitempty"`
}

// AssistantMessage represents an assistant message
type AssistantMessage struct {
	Model   string         `json:"model,omitempty"`
	Content []ContentBlock `json:"content,omitempty"`
}

// UserMessage represents a user message (tool results)
type UserMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content,omitempty"`
}

// ContentBlock represents a content block in a message
type ContentBlock struct {
	Type string `json:"type"`

	// For text content
	Text string `json:"text,omitempty"`

	// For tool_use content
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// For tool_result content
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

// ParsedEvent is a human-readable parsed event
type ParsedEvent struct {
	Type          string // "text", "tool_call", "tool_result", "result", "error"
	Summary       string // Short description
	Details       string // Full details (optional)
	Highlight     bool
	HighlightKind string // "pass", "fail", "commit", "close"
}

func applyHighlight(e *ParsedEvent) {
	s := strings.ToLower(e.Summary)
	switch {
	case strings.Contains(s, "bd close"):
		e.Highlight = true
		e.HighlightKind = "close"
	case strings.Contains(s, "passed") || strings.HasPrefix(s, "ok "):
		e.Highlight = true
		e.HighlightKind = "pass"
	case strings.Contains(s, "fail") || strings.Contains(s, "error"):
		e.Highlight = true
		e.HighlightKind = "fail"
	case strings.HasPrefix(s, "feat:") || strings.HasPrefix(s, "fix:") || strings.HasPrefix(s, "chore:") || strings.Contains(s, "commit"):
		e.Highlight = true
		e.HighlightKind = "commit"
	}
}

// ParseClaudeOutput parses the entire stream-json output and returns parsed events
func ParseClaudeOutput(rawOutput string) []ParsedEvent {
	var events []ParsedEvent
	lines := strings.Split(rawOutput, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parsed := ParseStreamLine(line)
		if parsed != nil {
			events = append(events, *parsed)
		}
	}

	return events
}

// ParseStreamLine parses a single JSON line from stream-json output
func ParseStreamLine(line string) *ParsedEvent {
	var evt ClaudeStreamEvent
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		return &ParsedEvent{
			Type:    "error",
			Summary: "Failed to parse JSON",
			Details: line[:min(100, len(line))],
		}
	}

	var event *ParsedEvent
	switch evt.Type {
	case "assistant":
		event = parseAssistantMessage(evt.Message)
	case "user":
		event = parseUserMessage(evt.Message)
	case "result":
		event = parseResultEvent(evt)
	default:
		return nil
	}

	if event != nil {
		applyHighlight(event)
	}
	return event
}

func parseAssistantMessage(raw json.RawMessage) *ParsedEvent {
	if raw == nil {
		return nil
	}

	var msg AssistantMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}

	var texts []string
	var tools []string

	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				// Truncate long text for summary
				text := strings.TrimSpace(block.Text)
				if len(text) > 200 {
					texts = append(texts, text[:200]+"...")
				} else {
					texts = append(texts, text)
				}
			}
		case "tool_use":
			// Extract tool name and brief input summary
			toolSummary := block.Name
			if block.Input != nil {
				inputStr := string(block.Input)
				if len(inputStr) > 80 {
					inputStr = inputStr[:80] + "..."
				}
				toolSummary = fmt.Sprintf("%s(%s)", block.Name, inputStr)
			}
			tools = append(tools, toolSummary)
		}
	}

	if len(tools) > 0 {
		return &ParsedEvent{
			Type:    "tool_call",
			Summary: "🔧 " + strings.Join(tools, ", "),
		}
	}

	if len(texts) > 0 {
		return &ParsedEvent{
			Type:    "text",
			Summary: "💬 " + strings.Join(texts, "\n"),
		}
	}

	return nil
}

func parseUserMessage(raw json.RawMessage) *ParsedEvent {
	if raw == nil {
		return nil
	}

	var msg UserMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil
	}

	for _, block := range msg.Content {
		if block.Type == "tool_result" {
			content := block.Content
			if len(content) > 150 {
				content = content[:150] + "..."
			}
			// Check if it looks like an error
			isError := strings.Contains(strings.ToLower(content), "error") ||
				strings.Contains(content, "Exit code 1")

			prefix := "📥"
			if isError {
				prefix = "❌"
			}

			return &ParsedEvent{
				Type:    "tool_result",
				Summary: fmt.Sprintf("%s %s", prefix, content),
			}
		}
	}

	return nil
}

func parseResultEvent(event ClaudeStreamEvent) *ParsedEvent {
	status := "✅ Success"
	if event.IsError {
		status = "❌ Error"
	}

	durationSec := float64(event.DurationMs) / 1000.0
	return &ParsedEvent{
		Type:    "result",
		Summary: fmt.Sprintf("%s | Duration: %.1fs | Turns: %d", status, durationSec, event.NumTurns),
	}
}

// FormatParsedEvents formats parsed events for display
func FormatParsedEvents(events []ParsedEvent) string {
	var b strings.Builder

	for i, event := range events {
		// Add separator between significant events
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(event.Summary)
		if event.Details != "" {
			b.WriteString("\n  ")
			b.WriteString(event.Details)
		}
	}

	return b.String()
}

// ExtractFullText extracts all text content from stream-json output without truncation
// Used for parsing Ralph status blocks that may be at the end of long messages
func ExtractFullText(rawOutput string) string {
	var texts []string
	lines := strings.Split(rawOutput, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var event ClaudeStreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.Type != "assistant" || event.Message == nil {
			continue
		}

		var msg AssistantMessage
		if err := json.Unmarshal(event.Message, &msg); err != nil {
			continue
		}

		for _, block := range msg.Content {
			if block.Type == "text" && block.Text != "" {
				texts = append(texts, block.Text)
			}
		}
	}

	return strings.Join(texts, "\n")
}

// ExtractKeyActivity extracts just the key activity (tool calls and results) for homebase
func ExtractKeyActivity(events []ParsedEvent) string {
	var b strings.Builder
	var lastType string

	for _, event := range events {
		// Skip consecutive tool results to reduce noise
		if event.Type == "tool_result" && lastType == "tool_result" {
			continue
		}

		switch event.Type {
		case "tool_call":
			b.WriteString("  " + event.Summary + "\n")
		case "text":
			// Only include short text snippets
			if len(event.Summary) < 100 {
				b.WriteString("  " + event.Summary + "\n")
			}
		case "result":
			b.WriteString("  " + event.Summary + "\n")
		}

		lastType = event.Type
	}

	return b.String()
}
