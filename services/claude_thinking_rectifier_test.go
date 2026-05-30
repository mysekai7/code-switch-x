package services

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestShouldRectifyClaudeThinkingSignatureErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "invalid signature in thinking block",
			body: `{"error":{"message":"messages.1.content.0: Invalid signature in thinking block"}}`,
			want: true,
		},
		{
			name: "thought signature invalid",
			body: `{"error":{"message":"Unable to submit request because Thought signature is not valid"}}`,
			want: true,
		},
		{
			name: "expected thinking before tool use",
			body: `{"error":{"message":"messages.3.content.0.type: Expected thinking or redacted_thinking, but found tool_use"}}`,
			want: true,
		},
		{
			name: "unrelated",
			body: `{"error":{"message":"rate limit exceeded"}}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRectifyClaudeThinkingSignature([]byte(tt.body)); got != tt.want {
				t.Fatalf("shouldRectifyClaudeThinkingSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectifyClaudeThinkingSignatureRequest(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4",
		"max_tokens": 4096,
		"thinking": {"type": "enabled", "budget_tokens": 2048},
		"messages": [
			{
				"role": "assistant",
				"content": [
					{"type": "thinking", "thinking": "Need tools", "signature": "sig-thinking"},
					{"type": "redacted_thinking", "data": "opaque", "signature": "sig-redacted"},
					{"type": "tool_use", "id": "toolu_1", "name": "read_file", "input": {"path": "README.md"}, "signature": "sig-tool"},
					{"type": "text", "text": "reading", "signature": "sig-text"}
				]
			}
		]
	}`)

	got, applied, err := rectifyClaudeThinkingSignatureRequest(body)
	if err != nil {
		t.Fatalf("rectifyClaudeThinkingSignatureRequest() error = %v", err)
	}
	if !applied {
		t.Fatalf("rectifyClaudeThinkingSignatureRequest() applied = false, want true")
	}
	if gjson.GetBytes(got, "thinking").Exists() {
		t.Fatalf("top-level thinking still exists after rectification: %s", string(got))
	}
	content := gjson.GetBytes(got, "messages.0.content")
	if gotLen := len(content.Array()); gotLen != 2 {
		t.Fatalf("content length = %d, want 2; body=%s", gotLen, string(got))
	}
	if gotType := content.Array()[0].Get("type").String(); gotType != "tool_use" {
		t.Fatalf("first content type = %q, want tool_use; body=%s", gotType, string(got))
	}
	if strings.Contains(string(got), "signature") {
		t.Fatalf("rectified request still contains signature: %s", string(got))
	}
}

func TestShouldRectifyClaudeThinkingBudgetErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "thinking budget below minimum",
			body: `{"error":{"message":"thinking.budget_tokens: Input should be greater than or equal to 1024"}}`,
			want: true,
		},
		{
			name: "budget max token relation is not the ccswitch rectifier trigger",
			body: `{"error":{"message":"budget_tokens must be less than max_tokens"}}`,
			want: false,
		},
		{
			name: "unrelated",
			body: `{"error":{"message":"invalid model"}}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRectifyClaudeThinkingBudget([]byte(tt.body)); got != tt.want {
				t.Fatalf("shouldRectifyClaudeThinkingBudget() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRectifyClaudeThinkingBudgetRequest(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4",
		"max_tokens": 1024,
		"thinking": {"type": "disabled", "budget_tokens": 512},
		"messages": [{"role": "user", "content": "hi"}]
	}`)

	got, applied, err := rectifyClaudeThinkingBudgetRequest(body)
	if err != nil {
		t.Fatalf("rectifyClaudeThinkingBudgetRequest() error = %v", err)
	}
	if !applied {
		t.Fatalf("rectifyClaudeThinkingBudgetRequest() applied = false, want true")
	}
	if gotType := gjson.GetBytes(got, "thinking.type").String(); gotType != "enabled" {
		t.Fatalf("thinking.type = %q, want enabled; body=%s", gotType, string(got))
	}
	if gotBudget := gjson.GetBytes(got, "thinking.budget_tokens").Int(); gotBudget != 32000 {
		t.Fatalf("thinking.budget_tokens = %d, want 32000; body=%s", gotBudget, string(got))
	}
	if gotMax := gjson.GetBytes(got, "max_tokens").Int(); gotMax != 64000 {
		t.Fatalf("max_tokens = %d, want 64000; body=%s", gotMax, string(got))
	}
}

func TestRectifyClaudeThinkingBudgetKeepsAdaptiveThinking(t *testing.T) {
	body := []byte(`{
		"model": "claude-sonnet-4",
		"max_tokens": 1024,
		"thinking": {"type": "adaptive", "budget_tokens": 512},
		"messages": [{"role": "user", "content": "hi"}]
	}`)

	got, applied, err := rectifyClaudeThinkingBudgetRequest(body)
	if err != nil {
		t.Fatalf("rectifyClaudeThinkingBudgetRequest() error = %v", err)
	}
	if applied {
		t.Fatalf("rectifyClaudeThinkingBudgetRequest() applied = true, want false; body=%s", string(got))
	}
	if string(got) != string(body) {
		t.Fatalf("adaptive body changed:\ngot=%s\nwant=%s", string(got), string(body))
	}
}
