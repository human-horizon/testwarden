package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNew_InfoLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{Output: &buf, Level: LevelInfo})
	logger.Info("hello", "key", "value")

	out := buf.String()
	if !strings.Contains(out, "hello") {
		t.Errorf("expected message in output, got: %s", out)
	}
	if !strings.Contains(out, "key=value") {
		t.Errorf("expected attr in output, got: %s", out)
	}
}

func TestNew_JSON(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{Output: &buf, Level: LevelDebug, JSON: true})
	logger.Info("test", "k", "v")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, buf.String())
	}
	if record["msg"] != "test" {
		t.Errorf("expected msg=test, got %v", record["msg"])
	}
}

func TestNew_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Options{Output: &buf, Level: LevelWarn})
	logger.Info("info-not-shown")
	logger.Warn("warn-shown")
	logger.Error("error-shown")

	out := buf.String()
	if strings.Contains(out, "info-not-shown") {
		t.Error("info should be filtered")
	}
	if !strings.Contains(out, "warn-shown") {
		t.Error("warn should appear")
	}
	if !strings.Contains(out, "error-shown") {
		t.Error("error should appear")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in   string
		want Level
	}{
		{"debug", LevelDebug},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"unknown", LevelInfo},
	}
	for _, tt := range tests {
		if got := ParseLevel(tt.in); got != tt.want {
			t.Errorf("ParseLevel(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestColorHandler(t *testing.T) {
	var buf bytes.Buffer
	h := NewColorHandler(&buf, nil)
	if !h.Enabled(nil, slogLevelInfo()) {
		t.Error("expected info to be enabled by default")
	}
	if err := h.Handle(nil, newRecord("test message", "key", "val")); err != nil {
		t.Fatalf("handle: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "test message") {
		t.Errorf("missing message in: %s", out)
	}
	if !strings.Contains(out, colorReset) {
		t.Error("missing color reset")
	}
}
