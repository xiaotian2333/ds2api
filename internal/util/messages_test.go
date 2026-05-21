package util

import (
	"strings"
	"testing"

	"ds2api/internal/config"
)

func TestMessagesPrepareBasic(t *testing.T) {
	messages := []map[string]any{{"role": "user", "content": "Hello"}}
	got := MessagesPrepare(messages)
	if got == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !strings.HasPrefix(got, "用户：") {
		t.Fatalf("expected user marker at the start, got %q", got)
	}
	if !strings.Contains(got, "Hello") || !strings.HasSuffix(got, "助手：") {
		t.Fatalf("unexpected prompt: %q", got)
	}
}

func TestMessagesPrepareRoles(t *testing.T) {
	messages := []map[string]any{
		{"role": "system", "content": "You are helper"},
		{"role": "user", "content": "Hi"},
		{"role": "assistant", "content": "Hello"},
		{"role": "tool", "content": "Search results"},
		{"role": "user", "content": "How are you"},
	}
	got := MessagesPrepare(messages)
	if !contains(got, "You are helper") || !contains(got, "用户：Hi") {
		t.Fatalf("expected system/user content in %q", got)
	}
	if !contains(got, "用户：Hi助手：Hello") {
		t.Fatalf("expected user/assistant separation in %q", got)
	}
	if !contains(got, "助手：Hello") {
		t.Fatalf("expected assistant marker in %q", got)
	}
	if !contains(got, "系统提示：此环境不可调用工具，调用已被拦截") {
		t.Fatalf("expected tool interception notice in %q", got)
	}
	if !contains(got, "用户：How are you") {
		t.Fatalf("expected user marker in %q", got)
	}
	if !contains(got, "助手：") {
		t.Fatalf("expected assistant marker in %q", got)
	}
	if !contains(got, "用户：") {
		t.Fatalf("expected user marker in %q", got)
	}
}

func TestMessagesPrepareObjectContent(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": map[string]any{"temp": 18, "ok": true}},
	}
	got := MessagesPrepare(messages)
	if !contains(got, `"temp":18`) || !contains(got, `"ok":true`) {
		t.Fatalf("expected serialized object content, got %q", got)
	}
}

func TestMessagesPrepareArrayTextVariants(t *testing.T) {
	messages := []map[string]any{
		{
			"role": "user",
			"content": []any{
				map[string]any{"type": "output_text", "text": "line1"},
				map[string]any{"type": "input_text", "text": "line2"},
				map[string]any{"type": "image_url", "image_url": "https://example.com/a.png"},
			},
		},
	}
	got := MessagesPrepare(messages)
	if !contains(got, "line1\nline2") {
		t.Fatalf("unexpected content from text variants: %q", got)
	}
}

func TestConvertClaudeToDeepSeek(t *testing.T) {
	store := config.LoadStore()
	req := map[string]any{
		"model":    "claude-opus-4-6",
		"messages": []any{map[string]any{"role": "user", "content": "Hi"}},
		"system":   "You are helpful",
		"stream":   true,
	}
	out := ConvertClaudeToDeepSeek(req, store)
	if out["model"] == "" {
		t.Fatal("expected mapped model")
	}
	msgs, ok := out["messages"].([]any)
	if !ok || len(msgs) == 0 {
		t.Fatal("expected messages")
	}
	first, _ := msgs[0].(map[string]any)
	if first["role"] != "system" {
		t.Fatalf("expected first message system, got %#v", first)
	}
}

func TestConvertClaudeToDeepSeekUsesGlobalAliasResolution(t *testing.T) {
	store := config.LoadStore()
	req := map[string]any{
		"model":    "claude-3-5-sonnet-latest",
		"messages": []any{map[string]any{"role": "user", "content": "Hi"}},
	}
	out := ConvertClaudeToDeepSeek(req, store)
	if out["model"] != "deepseek-v4-flash" {
		t.Fatalf("expected global alias resolution, got model=%q", out["model"])
	}
}

func TestConvertClaudeToDeepSeekUsesNoThinkingAliasResolution(t *testing.T) {
	store := config.LoadStore()
	req := map[string]any{
		"model":    "claude-sonnet-4-6-nothinking",
		"messages": []any{map[string]any{"role": "user", "content": "Hi"}},
	}
	out := ConvertClaudeToDeepSeek(req, store)
	if out["model"] != "deepseek-v4-flash-nothinking" {
		t.Fatalf("expected noThinking alias resolution, got model=%q", out["model"])
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
