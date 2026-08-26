// Package report formats check/fix results for humans and CI.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Issue is a single problem found during check.
type Issue struct {
	Type     string  `json:"type"`     // "coverage", "mock", "gap"
	Severity string  `json:"severity"` // "error", "warning"
	Message  string  `json:"message"`
	File     string  `json:"file,omitempty"`
	Line     int     `json:"line,omitempty"`
	Detail   string  `json:"detail,omitempty"`
	Value    float64 `json:"value,omitempty"`
}

// Result is the full check result.
type Result struct {
	Language  string  `json:"language"`
	Coverage  float64 `json:"coverage_percent"`
	Threshold int     `json:"threshold"`
	Passed    bool    `json:"passed"`
	Issues    []Issue `json:"issues"`
}

// PrintText writes a human-readable report to w.
func PrintText(w io.Writer, results []*Result) {
	for _, r := range results {
		fmt.Fprintf(w, "═══ %s ═══\n", strings.ToUpper(r.Language))
		fmt.Fprintf(w, "Coverage: %.2f%% (threshold: %d%%)\n", r.Coverage, r.Threshold)
		if r.Passed {
			fmt.Fprintln(w, "Status: ✓ PASS")
		} else {
			fmt.Fprintln(w, "Status: ✗ FAIL")
		}
		if len(r.Issues) > 0 {
			fmt.Fprintln(w, "\nIssues:")
			for _, issue := range r.Issues {
				location := issue.File
				if issue.Line > 0 {
					location = fmt.Sprintf("%s:%d", issue.File, issue.Line)
				}
				if location != "" {
					fmt.Fprintf(w, "  [%s] %s\n    → %s (%s)\n", issue.Severity, issue.Type, issue.Message, location)
				} else {
					fmt.Fprintf(w, "  [%s] %s\n    → %s\n", issue.Severity, issue.Type, issue.Message)
				}
			}
		}
		fmt.Fprintln(w)
	}
}

// PrintJSON writes a JSON report to w.
func PrintJSON(w io.Writer, results []*Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}
