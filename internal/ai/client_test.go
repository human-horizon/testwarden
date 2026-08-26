package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestNew(t *testing.T) {
	c := New(Config{Endpoint: "http://x", Model: "m", Timeout: 60, MaxTokens: 1000})
	if c.model != "m" {
		t.Errorf("expected model m, got %s", c.model)
	}
	if c.timeout.Seconds() != 60 {
		t.Errorf("expected 60s, got %v", c.timeout)
	}
	if c.maxTokens != 1000 {
		t.Errorf("expected 1000, got %d", c.maxTokens)
	}
}

func TestFix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "```go\npackage foo\n\nfunc Bar() {}\n```",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(Config{Endpoint: server.URL, Model: "test-model", MaxTokens: 1000})

	out, err := c.Fix(context.Background(),
		"low coverage", "coverage", "add tests", "foo.go", "go", "package foo",
	)
	if err != nil {
		t.Fatalf("fix: %v", err)
	}
	if !strings.Contains(out, "package foo") {
		t.Errorf("unexpected response: %s", out)
	}
}

func TestFix_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"choices": []any{}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(Config{Endpoint: server.URL, Model: "m"})
	_, err := c.Fix(context.Background(), "p", "default", "r", "f", "go", "x")
	if err == nil {
		t.Error("expected error on empty choices")
	}
}

func TestFix_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	c := New(Config{Endpoint: server.URL, Model: "m"})
	_, err := c.Fix(context.Background(), "p", "default", "r", "f", "go", "x")
	if err == nil {
		t.Error("expected error on 500")
	}
}

func TestFix_DefaultPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		messages, _ := req["messages"].([]any)
		first, _ := messages[0].(map[string]any)
		content, _ := first["content"].(string)
		if !strings.Contains(content, "senior test engineer") {
			t.Errorf("expected default prompt, got: %s", content)
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "```\nx := 1\n```"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(Config{Endpoint: server.URL, Model: "m"})
	_, _ = c.Fix(context.Background(), "p", "unknown_type", "r", "f", "go", "x")
}

func TestFix_DefaultMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.MaxTokens != 4096 {
			t.Errorf("expected default 4096, got %d", req.MaxTokens)
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "ok"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(Config{Endpoint: server.URL, Model: "m"})
	_, _ = c.Fix(context.Background(), "p", "default", "r", "f", "go", "x")
}
