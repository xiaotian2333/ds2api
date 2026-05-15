package claude

import (
	"fmt"
	"strings"
)

func normalizeClaudeMessages(messages []any) []any {
	out := make([]any, 0, len(messages))
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", msg["role"])))
		switch content := msg["content"].(type) {
		case []any:
			textParts := make([]string, 0, len(content))
			flushText := func() {
				if len(textParts) == 0 {
					return
				}
				out = append(out, map[string]any{
					"role":    role,
					"content": strings.Join(textParts, "\n"),
				})
				textParts = textParts[:0]
			}
			for _, block := range content {
				b, ok := block.(map[string]any)
				if !ok {
					continue
				}
				typeStr := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", b["type"])))
				switch typeStr {
				case "text":
					if t, ok := b["text"].(string); ok {
						textParts = append(textParts, t)
					}
				default:
					if raw := strings.TrimSpace(formatClaudeUnknownBlockForPrompt(b)); raw != "" {
						textParts = append(textParts, raw)
					}
				}
			}
			flushText()
		default:
			copied := cloneMap(msg)
			out = append(out, copied)
		}
	}
	return out
}

// formatClaudeBlockRaw is kept for compatibility.
func formatClaudeBlockRaw(block map[string]any) string {
	return formatClaudeUnknownBlockForPrompt(block)
}