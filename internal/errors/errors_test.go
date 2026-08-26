package errors

import (
	stderrors "errors"
	"fmt"
	"testing"
)

func TestError_Message(t *testing.T) {
	e := New(CodeConfig, "bad config")
	if got := e.Error(); got != "[CONFIG] bad config" {
		t.Errorf("got %q", got)
	}
}

func TestError_WithCause(t *testing.T) {
	cause := fmt.Errorf("underlying")
	e := Wrap(CodeCoverage, "parse failed", cause)
	if got := e.Error(); got != "[COVERAGE] parse failed: underlying" {
		t.Errorf("got %q", got)
	}
	if e.Unwrap() != cause {
		t.Error("Unwrap mismatch")
	}
}

func TestError_Context(t *testing.T) {
	e := New(CodeAI, "timeout").
		WithContext("file", "foo.go").
		WithContext("retry", 3)
	if e.Context["file"] != "foo.go" {
		t.Error("missing context")
	}
	if e.Context["retry"] != 3 {
		t.Error("missing retry")
	}
}

func TestErrorsIs(t *testing.T) {
	sentinel := fmt.Errorf("sentinel")
	e := Wrap(CodeNetwork, "io failed", sentinel)
	if !stderrors.Is(e, sentinel) {
		t.Error("expected errors.Is to find sentinel")
	}
}

func TestCodeOf(t *testing.T) {
	e := New(CodeAI, "x")
	if CodeOf(e) != CodeAI {
		t.Error("CodeOf mismatch")
	}
	if CodeOf(nil) != "" {
		t.Error("CodeOf(nil) should be empty")
	}
}

func TestIsCode(t *testing.T) {
	e := Wrap(CodeConfig, "x", stderrors.New("inner"))
	if !IsCode(e, CodeConfig) {
		t.Error("expected IsCode match")
	}
	if IsCode(e, CodeAI) {
		t.Error("expected mismatch")
	}
}
