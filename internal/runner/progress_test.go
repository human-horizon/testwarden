package runner

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/HumanHorizon/testwarden/internal/config"
	"github.com/HumanHorizon/testwarden/internal/report"
)

func TestNopProgress(t *testing.T) {
	p := nopProgress{}
	p.AnalyseStarted()
	p.FixStarted(3)
	p.IssueStarted(0, 3, "coverage", "foo.go")
	p.StreamStarted("ai")
	p.StreamChunk("x")
	p.StreamFinished()
	p.Done(nil)
	p.Error(nil)
	if err := p.Wait(); err != nil {
		t.Errorf("Wait: %v", err)
	}
}

func TestPlainProgress_WritesOutput(t *testing.T) {
	var buf bytes.Buffer
	p := &plainProgress{out: &buf}
	p.AnalyseStarted()
	p.FixStarted(2)
	p.IssueStarted(0, 2, "coverage", "foo.go")
	p.Error(nil)
	p.Wait()

	out := buf.String()
	if !strings.Contains(out, "Analysing") {
		t.Error("missing 'Analysing'")
	}
	if !strings.Contains(out, "Found 2 issue") {
		t.Error("missing issue count")
	}
	if !strings.Contains(out, "[1/2]") {
		t.Error("missing progress [1/2]")
	}
}

func TestNewProgress_TUIOff(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Cfg: config.Default(),
		Out: &buf,
		TUI: false,
	}
	p := newProgress(opts)
	if _, ok := p.(*plainProgress); !ok {
		t.Errorf("expected plainProgress when TUI=false, got %T", p)
	}
}

func TestNewProgress_TUIOnButNoTerminal(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Cfg: config.Default(),
		Out: &buf,
		TUI: true,
	}
	p := newProgress(opts)
	// TUI=true but stdout is buffer (not a terminal) → falls back to plain.
	if _, ok := p.(*plainProgress); !ok {
		t.Errorf("expected plainProgress in non-terminal env even with TUI=true, got %T", p)
	}
}

func TestRunCheck_QuietSuppressesOutput(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Cfg:   config.Default(),
		Root:  t.TempDir(),
		Quiet: true,
		Out:   &buf,
	}
	opts.Cfg.Coverage.UnitCommand = ""
	opts.Cfg.Languages = []string{"go"}

	code, err := RunCheck(context.Background(), opts)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if code == 0 {
		t.Error("expected non-zero exit (no coverage.out)")
	}
	if buf.Len() != 0 {
		t.Errorf("quiet should suppress output, got: %s", buf.String())
	}
}

func TestRunFix_IssueProgress(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		Cfg:    config.Default(),
		Root:   t.TempDir(),
		TUI:    false,
		Out:    &buf,
		DryRun: true,
	}
	opts.Cfg.Coverage.UnitCommand = ""
	opts.Cfg.Languages = []string{"go"}

	results := []*report.Result{{
		Language:  "go",
		Coverage:  50.0,
		Threshold: 80,
		Passed:    false,
		Issues: []report.Issue{
			{Type: "coverage", File: "foo.go", Message: "below threshold"},
		},
	}}
	_ = results

	progress := newProgress(opts)
	progress.AnalyseStarted()
	progress.FixStarted(1)
	progress.IssueStarted(0, 1, "coverage", "foo.go")
	progress.StreamStarted("ai")
	progress.StreamChunk("package foo")
	progress.StreamFinished()
	progress.Done(results)
	progress.Wait()

	out := buf.String()
	if !strings.Contains(out, "[1/1]") {
		t.Errorf("expected '[1/1]' in output, got: %s", out)
	}
}

func TestOptions_Validate_MissingCfg(t *testing.T) {
	opts := Options{Root: t.TempDir()}
	if err := opts.validate(); err == nil {
		t.Error("expected error for missing config")
	}
}

func TestOptions_Validate_BadRoot(t *testing.T) {
	opts := Options{
		Cfg:  config.Default(),
		Root: "/nonexistent/path",
	}
	if err := opts.validate(); err == nil {
		t.Error("expected error for non-existent root")
	}
}

func TestOptions_Validate_OK(t *testing.T) {
	opts := Options{
		Cfg:  config.Default(),
		Root: t.TempDir(),
	}
	if err := opts.validate(); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}
