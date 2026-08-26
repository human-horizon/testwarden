package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFixStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

		chunks := []string{"```go\n", "package foo\n", "\nfunc Bar() {}\n", "```"}
		for _, c := range chunks {
			data, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{
					{"delta": map[string]string{"content": c}},
				},
			})
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	c := New(Config{Endpoint: server.URL, Model: "test"})

	var received []string
	out, err := c.FixStream(context.Background(),
		"low coverage", "coverage", "rule", "foo.go", "go", "package foo",
		func(chunk string) { received = append(received, chunk) },
	)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if !strings.Contains(out, "package foo") {
		t.Errorf("unexpected output: %s", out)
	}
	if len(received) == 0 {
		t.Error("expected chunks to be received")
	}
}

func TestFixStream_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	c := New(Config{Endpoint: server.URL, Model: "test"})
	_, err := c.FixStream(context.Background(), "p", "coverage", "r", "f", "go", "x", nil)
	if err == nil {
		t.Error("expected error on empty stream")
	}
}

func TestFix_PromptByType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		messages, _ := req["messages"].([]any)
		if len(messages) < 1 {
			t.Fatal("expected system message")
		}
		first := messages[0].(map[string]any)
		content, _ := first["content"].(string)
		if !strings.Contains(content, "low test coverage") {
			t.Errorf("expected coverage prompt, got: %s", content)
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "```\nx := 1\n```"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := New(Config{Endpoint: server.URL, Model: "test"})
	_, _ = c.Fix(context.Background(), "p", "coverage", "r", "f", "go", "x")
}
