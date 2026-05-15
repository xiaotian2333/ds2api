package responses

import (
	"encoding/json"
	"strings"

	openaifmt "ds2api/internal/format/openai"

	"github.com/google/uuid"
)

func (s *responsesStreamRuntime) nextSequence() int {
	s.sequence++
	return s.sequence
}

func (s *responsesStreamRuntime) sendEvent(event string, payload map[string]any) {
	if payload == nil {
		payload = map[string]any{}
	}
	if _, ok := payload["sequence_number"]; !ok {
		payload["sequence_number"] = s.nextSequence()
	}
	b, _ := json.Marshal(payload)
	_, _ = s.w.Write([]byte("event: " + event + "\n"))
	_, _ = s.w.Write([]byte("data: "))
	_, _ = s.w.Write(b)
	_, _ = s.w.Write([]byte("\n\n"))
	if s.canFlush {
		_ = s.rc.Flush()
	}
}

func (s *responsesStreamRuntime) sendCreated() {
	s.sendEvent("response.created", openaifmt.BuildResponsesCreatedPayload(s.responseID, s.model))
}

func (s *responsesStreamRuntime) sendDone() {
	_, _ = s.w.Write([]byte("data: [DONE]\n\n"))
	if s.canFlush {
		_ = s.rc.Flush()
	}
}

func (s *responsesStreamRuntime) ensureMessageItemID() string {
	if strings.TrimSpace(s.messageItemID) != "" {
		return s.messageItemID
	}
	s.messageItemID = "msg_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	return s.messageItemID
}

func (s *responsesStreamRuntime) ensureMessageOutputIndex() int {
	if s.messageOutputID >= 0 {
		return s.messageOutputID
	}
	s.messageOutputID = s.allocateOutputIndex()
	return s.messageOutputID
}

func (s *responsesStreamRuntime) allocateOutputIndex() int {
	idx := s.nextOutputID
	s.nextOutputID++
	return idx
}

func (s *responsesStreamRuntime) ensureMessageItemAdded() {
	if s.messageAdded {
		return
	}
	itemID := s.ensureMessageItemID()
	item := map[string]any{
		"id":     itemID,
		"type":   "message",
		"role":   "assistant",
		"status": "in_progress",
	}
	s.sendEvent(
		"response.output_item.added",
		openaifmt.BuildResponsesOutputItemAddedPayload(s.responseID, itemID, s.ensureMessageOutputIndex(), item),
	)
	s.messageAdded = true
}

func (s *responsesStreamRuntime) ensureMessageContentPartAdded() {
	if s.messagePartAdded {
		return
	}
	s.ensureMessageItemAdded()
	s.sendEvent(
		"response.content_part.added",
		openaifmt.BuildResponsesContentPartAddedPayload(
			s.responseID,
			s.ensureMessageItemID(),
			s.ensureMessageOutputIndex(),
			0,
			map[string]any{"type": "output_text", "text": ""},
		),
	)
	s.messagePartAdded = true
}

func (s *responsesStreamRuntime) emitTextDelta(content string) {
	if content == "" {
		return
	}
	s.ensureMessageContentPartAdded()
	s.visibleText.WriteString(content)
	s.sendEvent(
		"response.output_text.delta",
		openaifmt.BuildResponsesTextDeltaPayload(
			s.responseID,
			s.ensureMessageItemID(),
			s.ensureMessageOutputIndex(),
			0,
			content,
		),
	)
}

func (s *responsesStreamRuntime) closeMessageItem() {
	if !s.messageAdded {
		return
	}
	itemID := s.ensureMessageItemID()
	outputIndex := s.ensureMessageOutputIndex()
	text := s.visibleText.String()
	if s.messagePartAdded {
		s.sendEvent(
			"response.output_text.done",
			openaifmt.BuildResponsesTextDonePayload(
				s.responseID,
				itemID,
				outputIndex,
				0,
				text,
			),
		)
		s.sendEvent(
			"response.content_part.done",
			openaifmt.BuildResponsesContentPartDonePayload(
				s.responseID,
				itemID,
				outputIndex,
				0,
				map[string]any{"type": "output_text", "text": text},
			),
		)
		s.messagePartAdded = false
	}
	item := map[string]any{
		"id":     itemID,
		"type":   "message",
		"role":   "assistant",
		"status": "completed",
		"content": []map[string]any{
			{
				"type": "output_text",
				"text": text,
			},
		},
	}
	s.sendEvent(
		"response.output_item.done",
		openaifmt.BuildResponsesOutputItemDonePayload(s.responseID, itemID, outputIndex, item),
	)
}