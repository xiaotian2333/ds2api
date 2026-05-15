package promptcompat

import (
	"fmt"
	"strings"

	"ds2api/internal/config"
	"ds2api/internal/util"
)

type ConfigReader interface {
	ModelAliases() map[string]string
}

func NormalizeOpenAIChatRequest(store ConfigReader, req map[string]any, traceID string) (StandardRequest, error) {
	model, _ := req["model"].(string)
	messagesRaw, _ := req["messages"].([]any)
	if strings.TrimSpace(model) == "" || len(messagesRaw) == 0 {
		return StandardRequest{}, fmt.Errorf("request must include 'model' and 'messages'")
	}
	resolvedModel, ok := config.ResolveModel(store, model)
	if !ok {
		return StandardRequest{}, fmt.Errorf("model %q is not available", model)
	}
	defaultThinkingEnabled, searchEnabled, _ := config.GetModelConfig(resolvedModel)
	thinkingEnabled := util.ResolveThinkingEnabled(req, defaultThinkingEnabled)
	if config.IsNoThinkingModel(resolvedModel) {
		thinkingEnabled = false
	}
	responseModel := strings.TrimSpace(model)
	if responseModel == "" {
		responseModel = resolvedModel
	}
	finalPrompt, _ := BuildOpenAIPrompt(messagesRaw, req["tools"], traceID, thinkingEnabled)
	passThrough := collectOpenAIChatPassThrough(req)
	refFileIDs := CollectOpenAIRefFileIDs(req)

	return StandardRequest{
		Surface:         "openai_chat",
		RequestedModel:  strings.TrimSpace(model),
		ResolvedModel:   resolvedModel,
		ResponseModel:   responseModel,
		Messages:        messagesRaw,
		PromptTokenText: finalPrompt,
		FinalPrompt:     finalPrompt,
		Stream:          util.ToBool(req["stream"]),
		Thinking:        thinkingEnabled,
		Search:          searchEnabled,
		RefFileIDs:      refFileIDs,
		RefFileTokens:   estimateInlineFileTokens(req),
		PassThrough:     passThrough,
	}, nil
}

func NormalizeOpenAIResponsesRequest(store ConfigReader, req map[string]any, traceID string) (StandardRequest, error) {
	model, _ := req["model"].(string)
	model = strings.TrimSpace(model)
	if model == "" {
		return StandardRequest{}, fmt.Errorf("request must include 'model'")
	}
	resolvedModel, ok := config.ResolveModel(store, model)
	if !ok {
		return StandardRequest{}, fmt.Errorf("model %q is not available", model)
	}
	defaultThinkingEnabled, searchEnabled, _ := config.GetModelConfig(resolvedModel)
	thinkingEnabled := util.ResolveThinkingEnabled(req, defaultThinkingEnabled)
	if config.IsNoThinkingModel(resolvedModel) {
		thinkingEnabled = false
	}

	messagesRaw := ResponsesMessagesFromRequest(req)
	if len(messagesRaw) == 0 {
		return StandardRequest{}, fmt.Errorf("request must include 'input' or 'messages'")
	}
	finalPrompt, _ := BuildOpenAIPrompt(messagesRaw, req["tools"], traceID, thinkingEnabled)
	passThrough := collectOpenAIChatPassThrough(req)
	refFileIDs := CollectOpenAIRefFileIDs(req)

	return StandardRequest{
		Surface:         "openai_responses",
		RequestedModel:  model,
		ResolvedModel:   resolvedModel,
		ResponseModel:   model,
		Messages:        messagesRaw,
		PromptTokenText: finalPrompt,
		FinalPrompt:     finalPrompt,
		Stream:          util.ToBool(req["stream"]),
		Thinking:        thinkingEnabled,
		Search:          searchEnabled,
		RefFileIDs:      refFileIDs,
		RefFileTokens:   estimateInlineFileTokens(req),
		PassThrough:     passThrough,
	}, nil
}

func collectOpenAIChatPassThrough(req map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range []string{
		"temperature",
		"top_p",
		"max_tokens",
		"max_completion_tokens",
		"presence_penalty",
		"frequency_penalty",
		"stop",
	} {
		if v, ok := req[k]; ok {
			out[k] = v
		}
	}
	return out
}

// estimateInlineFileTokens extracts the byte count stashed by PreprocessInlineFileInputs
// and converts it to a conservative token estimate. Inline files are typically images or
// documents that the upstream model will process; we use bytes/3 (rather than bytes/4)
// as a slightly pessimistic approximation so the returned context token count stays
// safely above the real value.
func estimateInlineFileTokens(req map[string]any) int {
	raw, ok := req["_inline_file_bytes"]
	if !ok {
		return 0
	}
	var bytes int
	switch v := raw.(type) {
	case int:
		bytes = v
	case int64:
		bytes = int(v)
	case float64:
		bytes = int(v)
	default:
		return 0
	}
	if bytes <= 0 {
		return 0
	}
	return bytes / 3
}