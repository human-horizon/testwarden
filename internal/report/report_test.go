package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPrintText(t *testing.T) {
	results := []*Result{
		{
			Language:  "go",
			Coverage:  50.0,
			Threshold: 80,
			Passed:    false,
			Issues: []Issue{
				{Type: "coverage", Severity: "error", Message: "below threshold"},
				{Type: "mock", Severity: "warning", Message: "over-mocking", File: "foo_test.go", Line: 5},
			},
		},
	}

	var buf bytes.Buffer
	PrintText(&buf, results)

	out := buf.String()
	if !strings.Contains(out, "50.00%") {
		t.Error("missing coverage percentage")
	}
	if !strings.Contains(out, "FAIL") {
		t.Error("missing FAIL")
	}
	if !strings.Contains(out, "foo_test.go:5") {
		t.Error("missing file location")
	}
}

func TestPrintJSON(t *testing.T) {
	results := []*Result{
		{Language: "go", Coverage: 95, Threshold: 80, Passed: true},
	}

	var buf bytes.Buffer
	if err := PrintJSON(&buf, results); err != nil {
		t.Fatalf("print: %v", err)
	}

	var decoded []*Result
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded) != 1 || !decoded[0].Passed {
		t.Error("invalid decoded result")
	}
}
