package promptcompat

import (
	"ds2api/internal/prompt"
)

func BuildOpenAIPrompt(messagesRaw []any, toolsRaw any, traceID string, thinkingEnabled bool) (string, []string) {
	messages := NormalizeOpenAIMessagesForPrompt(messagesRaw, traceID)
	return prompt.MessagesPrepareWithThinking(messages, thinkingEnabled), nil
}

// BuildOpenAIPromptForAdapter exposes the OpenAI-compatible prompt building flow so
// other protocol adapters (for example Gemini) can reuse the same tool/history
// normalization logic and remain behavior-compatible with chat/completions.
func BuildOpenAIPromptForAdapter(messagesRaw []any, toolsRaw any, traceID string, thinkingEnabled bool) (string, []string) {
	return BuildOpenAIPrompt(messagesRaw, toolsRaw, traceID, thinkingEnabled)
}