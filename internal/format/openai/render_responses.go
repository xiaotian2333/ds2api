package openai

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

func BuildResponseObject(responseID, model, finalPrompt, finalThinking, finalText string, toolNames []string, toolsRaw any) map[string]any {
	return BuildResponseObjectWithToolCalls(responseID, model, finalPrompt, finalThinking, finalText, nil, nil)
}

func BuildResponseObjectWithToolCalls(responseID, model, finalPrompt, finalThinking, finalText string, detected []ParsedToolCall, toolsRaw any) map[string]any {
	exposedOutputText := finalText
	output := make([]any, 0, 2)
	content := make([]any, 0, 2)
	if finalThinking != "" {
		content = append([]any{map[string]any{
			"type": "reasoning",
			"text": finalThinking,
		}}, content...)
	}
	if strings.TrimSpace(finalText) != "" {
		content = append(content, map[string]any{
			"type": "output_text",
			"text": finalText,
		})
	}
	if strings.TrimSpace(finalText) == "" && strings.TrimSpace(finalThinking) != "" {
		exposedOutputText = finalThinking
	}
	output = append(output, map[string]any{
		"type":    "message",
		"id":      "msg_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		"role":    "assistant",
		"content": content,
	})
	return BuildResponseObjectFromItems(
		responseID,
		model,
		finalPrompt,
		finalThinking,
		finalText,
		output,
		exposedOutputText,
	)
}

func BuildResponseObjectFromItems(responseID, model, finalPrompt, finalThinking, finalText string, output []any, outputText string) map[string]any {
	if output == nil {
		output = []any{}
	}
	return map[string]any{
		"id":          responseID,
		"type":        "response",
		"object":      "response",
		"created_at":  time.Now().Unix(),
		"status":      "completed",
		"model":       model,
		"output":      output,
		"output_text": outputText,
		"usage":       BuildResponsesUsageForModel(model, finalPrompt, finalThinking, finalText, 0),
	}
}

// normalizeJSONString normalizes a JSON string by parsing and re-marshaling it.
func normalizeJSONString(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "{}"
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return raw
	}
	b, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return string(b)
}