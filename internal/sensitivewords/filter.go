package sensitivewords

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"ds2api/internal/config"
)

type Matcher struct {
	mu       sync.RWMutex
	patterns []*compiledPattern
	enabled  bool
}

type compiledPattern struct {
	raw string
	re  *regexp.Regexp
}

func NewMatcher() *Matcher {
	return &Matcher{}
}

func (m *Matcher) Update(cfg config.SensitiveWordsConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabled = cfg.Enabled
	m.patterns = m.patterns[:0]

	for _, raw := range cfg.Patterns {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		re, err := regexp.Compile(raw)
		if err != nil {
			return fmt.Errorf("敏感词正则编译失败 %q: %w", raw, err)
		}
		m.patterns = append(m.patterns, &compiledPattern{raw: raw, re: re})
	}
	return nil
}

func (m *Matcher) Check(text string) (matched bool, pattern string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled || len(m.patterns) == 0 {
		return false, ""
	}
	for _, cp := range m.patterns {
		if cp.re.MatchString(text) {
			return true, cp.raw
		}
	}
	return false, ""
}

// ExtractTextFromRequest 从请求中提取所有文本内容
func ExtractTextFromRequest(req map[string]any) []string {
	var texts []string
	if msgs, ok := req["messages"].([]any); ok {
		texts = append(texts, extractFromMessages(msgs)...)
	}
	if input, ok := req["input"]; ok {
		texts = append(texts, extractTextFromInput(input)...)
	}
	if contents, ok := req["contents"].([]any); ok {
		texts = append(texts, extractFromGeminiContents(contents)...)
	}
	if system, ok := req["system"].([]any); ok {
		texts = append(texts, extractFromClaudeSystem(system)...)
	}
	if system, ok := req["system"].(string); ok {
		texts = append(texts, system)
	}
	return texts
}

func extractFromMessages(msgs []any) []string {
	var texts []string
	for _, item := range msgs {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		texts = append(texts, extractContentFromMsg(msg)...)
	}
	return texts
}

func extractContentFromMsg(msg map[string]any) []string {
	var texts []string
	content := msg["content"]
	switch v := content.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			texts = append(texts, v)
		}
	case []any:
		for _, part := range v {
			p, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if t, ok := p["text"].(string); ok && strings.TrimSpace(t) != "" {
				texts = append(texts, t)
			}
		}
	}
	return texts
}

func extractTextFromInput(input any) []string {
	var texts []string
	switch v := input.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			texts = append(texts, v)
		}
	case []any:
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch mv := m["content"]; {
			case mv != nil:
				inner, ok := mv.([]any)
				if !ok {
					if s, ok := mv.(string); ok {
						texts = append(texts, s)
					}
					continue
				}
				for _, part := range inner {
					p, ok := part.(map[string]any)
					if !ok {
						continue
					}
					if t, ok := p["text"].(string); ok && strings.TrimSpace(t) != "" {
						texts = append(texts, t)
					}
				}
			case m["type"] == "message":
				if mc, ok := m["content"].([]any); ok {
					texts = append(texts, extractFromMessages(mc)...)
				}
			case m["role"] != nil:
				texts = append(texts, extractContentFromMsg(m)...)
			}
		}
	}
	return texts
}

func extractFromGeminiContents(contents []any) []string {
	var texts []string
	for _, item := range contents {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		parts, ok := m["parts"].([]any)
		if !ok {
			continue
		}
		for _, part := range parts {
			p, ok := part.(map[string]any)
			if !ok {
				continue
			}
			if t, ok := p["text"].(string); ok && strings.TrimSpace(t) != "" {
				texts = append(texts, t)
			}
		}
	}
	return texts
}

func extractFromClaudeSystem(system []any) []string {
	var texts []string
	for _, item := range system {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := m["text"].(string); ok && strings.TrimSpace(t) != "" {
			texts = append(texts, t)
		}
	}
	return texts
}
