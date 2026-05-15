package responses

import (
	"net/http"
	"strings"

	"ds2api/internal/assistantturn"
	openaifmt "ds2api/internal/format/openai"
	"ds2api/internal/httpapi/openai/shared"
	"ds2api/internal/responsehistory"
	"ds2api/internal/sse"
	streamengine "ds2api/internal/stream"
)

type responsesStreamRuntime struct {
	w        http.ResponseWriter
	rc       *http.ResponseController
	canFlush bool

	responseID    string
	model         string
	finalPrompt   string
	refFileTokens int
	traceID       string

	thinkingEnabled       bool
	searchEnabled         bool
	stripReferenceMarkers bool

	accumulator       shared.StreamAccumulator
	visibleText       strings.Builder
	responseMessageID int

	messageItemID     string
	messageOutputID   int
	nextOutputID      int
	messageAdded      bool
	messagePartAdded  bool
	sequence          int
	failed            bool
	finalErrorStatus  int
	finalErrorMessage string
	finalErrorCode    string

	persistResponse func(obj map[string]any)
	history         *responsehistory.Session
}

func newResponsesStreamRuntime(
	w http.ResponseWriter,
	rc *http.ResponseController,
	canFlush bool,
	responseID string,
	model string,
	finalPrompt string,
	thinkingEnabled bool,
	searchEnabled bool,
	stripReferenceMarkers bool,
	traceID string,
	persistResponse func(obj map[string]any),
	history *responsehistory.Session,
) *responsesStreamRuntime {
	return &responsesStreamRuntime{
		w:                     w,
		rc:                    rc,
		canFlush:              canFlush,
		responseID:            responseID,
		model:                 model,
		finalPrompt:           finalPrompt,
		thinkingEnabled:       thinkingEnabled,
		searchEnabled:         searchEnabled,
		stripReferenceMarkers: stripReferenceMarkers,
		traceID:               traceID,
		persistResponse:       persistResponse,
		history:               history,
		messageOutputID:       -1,
		accumulator: shared.StreamAccumulator{
			ThinkingEnabled:       thinkingEnabled,
			SearchEnabled:         searchEnabled,
			StripReferenceMarkers: stripReferenceMarkers,
		},
	}
}

func (s *responsesStreamRuntime) failResponse(status int, message, code string) {
	s.failed = true
	s.finalErrorStatus = status
	s.finalErrorMessage = message
	s.finalErrorCode = code
	failedResp := map[string]any{
		"id":          s.responseID,
		"type":        "response",
		"object":      "response",
		"model":       s.model,
		"status":      "failed",
		"status_code": status,
		"output":      []any{},
		"output_text": "",
		"error": map[string]any{
			"message": message,
			"type":    openAIErrorType(status),
			"code":    code,
			"param":   nil,
		},
	}
	if s.persistResponse != nil {
		s.persistResponse(failedResp)
	}
	if s.history != nil {
		s.history.Error(status, message, code, responsehistory.ThinkingForArchive(s.accumulator.RawThinking.String(), s.accumulator.ToolDetectionThinking.String(), s.accumulator.Thinking.String()), responsehistory.TextForArchive(s.accumulator.RawText.String(), s.accumulator.Text.String()))
	}
	s.sendEvent("response.failed", openaifmt.BuildResponsesFailedPayload(s.responseID, s.model, status, message, code))
	s.sendDone()
}

func (s *responsesStreamRuntime) markContextCancelled() {
	s.failed = true
	s.finalErrorStatus = 499
	s.finalErrorMessage = "request context cancelled"
	s.finalErrorCode = string(streamengine.StopReasonContextCancelled)
}

func (s *responsesStreamRuntime) finalize(finishReason string, deferEmptyOutput bool) bool {
	s.failed = false
	s.finalErrorStatus = 0
	s.finalErrorMessage = ""
	s.finalErrorCode = ""

	finalThinking := s.accumulator.Thinking.String()
	finalToolDetectionThinking := s.accumulator.ToolDetectionThinking.String()
	finalText := s.accumulator.Text.String()
	turn := assistantturn.BuildTurnFromStreamSnapshot(assistantturn.StreamSnapshot{
		RawText:               s.accumulator.RawText.String(),
		VisibleText:           finalText,
		RawThinking:           s.accumulator.RawThinking.String(),
		VisibleThinking:       finalThinking,
		DetectionThinking:     finalToolDetectionThinking,
		ContentFilter:         finishReason == "content_filter",
		ResponseMessageID:     s.responseMessageID,
	}, assistantturn.BuildOptions{
		Model:                 s.model,
		Prompt:                s.finalPrompt,
		RefFileTokens:         s.refFileTokens,
		SearchEnabled:         s.searchEnabled,
		StripReferenceMarkers: s.stripReferenceMarkers,
	})

	s.closeMessageItem()

	outcome := assistantturn.FinalizeTurn(turn, assistantturn.FinalizeOptions{})
	if outcome.ShouldFail {
		status, message, code := outcome.Error.Status, outcome.Error.Message, outcome.Error.Code
		if deferEmptyOutput {
			s.finalErrorStatus = status
			s.finalErrorMessage = message
			s.finalErrorCode = code
			return false
		}
		s.failResponse(status, message, code)
		return true
	}

	obj := buildCompletedResponseObject(s.responseID, s.model, s.finalPrompt, turn.Thinking, turn.Text, s.visibleText.String(), s.messageAdded, s.ensureMessageItemID, s.ensureMessageOutputIndex)
	if s.persistResponse != nil {
		s.persistResponse(obj)
	}
	if s.history != nil {
		s.history.Success(
			http.StatusOK,
			responsehistory.ThinkingForArchive(turn.RawThinking, turn.DetectionThinking, turn.Thinking),
			responsehistory.TextForArchive(turn.RawText, turn.Text),
			outcome.FinishReason,
			assistantturn.OpenAIResponsesUsage(turn),
		)
	}
	s.sendEvent("response.completed", openaifmt.BuildResponsesCompletedPayload(obj))
	s.sendDone()
	return true
}

func (s *responsesStreamRuntime) onParsed(parsed sse.LineResult) streamengine.ParsedDecision {
	if !parsed.Parsed {
		return streamengine.ParsedDecision{}
	}
	if parsed.ResponseMessageID > 0 {
		s.responseMessageID = parsed.ResponseMessageID
	}
	if parsed.ContentFilter || parsed.ErrorMessage != "" {
		return streamengine.ParsedDecision{Stop: true, StopReason: streamengine.StopReason("content_filter")}
	}
	if parsed.Stop {
		return streamengine.ParsedDecision{Stop: true}
	}

	batch := responsesDeltaBatch{runtime: s}
	accumulated := s.accumulator.Apply(parsed)
	for _, p := range accumulated.Parts {
		if p.Type == "thinking" {
			batch.append("reasoning", p.VisibleText)
			continue
		}
		if p.RawText == "" {
			continue
		}
		if p.CitationOnly {
			continue
		}
		batch.append("text", p.VisibleText)
	}

	batch.flush()
	if s.history != nil {
		s.history.Progress(
			responsehistory.ThinkingForArchive(s.accumulator.RawThinking.String(), s.accumulator.ToolDetectionThinking.String(), s.accumulator.Thinking.String()),
			responsehistory.TextForArchive(s.accumulator.RawText.String(), s.accumulator.Text.String()),
		)
	}
	return streamengine.ParsedDecision{ContentSeen: accumulated.ContentSeen}
}

func buildCompletedResponseObject(responseID, model, finalPrompt, finalThinking, finalText, visibleText string, messageAdded bool, ensureMessageItemID func() string, ensureMessageOutputIndex func() int) map[string]any {
	output := make([]any, 0, 1)
	content := make([]any, 0, 2)
	if finalThinking != "" {
		content = append(content, map[string]any{
			"type": "reasoning",
			"text": finalThinking,
		})
	}
	if finalText != "" {
		content = append(content, map[string]any{
			"type": "output_text",
			"text": finalText,
		})
	}
	if len(content) > 0 {
		output = append(output, map[string]any{
			"id":      ensureMessageItemID(),
			"type":    "message",
			"role":    "assistant",
			"status":  "completed",
			"content": content,
		})
	}

	outputText := visibleText
	if outputText == "" {
		if finalText != "" {
			outputText = finalText
		} else if finalThinking != "" {
			outputText = finalThinking
		}
	}

	return openaifmt.BuildResponseObjectFromItems(
		responseID,
		model,
		finalPrompt,
		finalThinking,
		finalText,
		output,
		outputText,
	)
}