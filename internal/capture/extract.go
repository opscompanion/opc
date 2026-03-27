package capture

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ReadTranscript reads a Claude Code NDJSON transcript and formats it as markdown.
func ReadTranscript(transcriptPath string) (string, error) {
	// Expand ~ if present
	if strings.HasPrefix(transcriptPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		transcriptPath = home + transcriptPath[1:]
	}

	f, err := os.Open(transcriptPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var b strings.Builder
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}

		role, content := extractTurnContent(raw)
		if role == "" || content == "" {
			continue
		}

		// Truncate very long turns to keep upload reasonable
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}

		b.WriteString(fmt.Sprintf("**%s**: %s\n\n", role, content))
	}

	return strings.TrimSpace(b.String()), nil
}

// extractTurnContent pulls role and text content from a transcript NDJSON line.
func extractTurnContent(raw map[string]json.RawMessage) (string, string) {
	// Handle "message" wrapper (common format)
	if msg, ok := raw["message"]; ok {
		var message struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		}
		if err := json.Unmarshal(msg, &message); err == nil && message.Role != "" {
			return message.Role, flattenContent(message.Content)
		}
	}

	// Handle direct role + content
	var role string
	if r, ok := raw["role"]; ok {
		json.Unmarshal(r, &role)
	}
	if role != "" {
		if c, ok := raw["content"]; ok {
			var content any
			json.Unmarshal(c, &content)
			return role, flattenContent(content)
		}
	}

	return "", ""
}

// flattenContent extracts text from various content formats (string, array of blocks, etc.)
func flattenContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			switch block := item.(type) {
			case string:
				parts = append(parts, block)
			case map[string]any:
				if text, ok := block["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, " ")
	}
	return ""
}
