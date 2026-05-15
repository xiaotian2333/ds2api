package shared

import "ds2api/internal/promptcompat"

func ApplyThinkingInjection(store ConfigReader, stdReq promptcompat.StandardRequest) promptcompat.StandardRequest {
	if store == nil || !store.ThinkingInjectionEnabled() || !stdReq.Thinking {
		return stdReq
	}
	messages, changed := promptcompat.AppendThinkingInjectionPromptToLatestUser(stdReq.Messages, store.ThinkingInjectionPrompt())
	if !changed {
		return stdReq
	}
	finalPrompt, _ := promptcompat.BuildOpenAIPrompt(messages, nil, "", stdReq.Thinking)
	stdReq.Messages = messages
	stdReq.FinalPrompt = finalPrompt
	return stdReq
}