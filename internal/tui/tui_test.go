package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModel_Init(t *testing.T) {
	m := New()
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a tick command")
	}
}

func TestModel_AnalyseStarted(t *testing.T) {
	m := New()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(AnalyseStartedMsg{})

	if m.state != StateAnalysing {
		t.Errorf("expected StateAnalysing, got %d", m.state)
	}
	if !strings.Contains(m.currentAction, "Analysing") {
		t.Errorf("expected action to mention analysing, got %s", m.currentAction)
	}
}

func TestModel_FixStarted(t *testing.T) {
	m := New()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(FixStartedMsg{TotalIssues: 3})

	if m.state != StateFixing {
		t.Errorf("expected StateFixing, got %d", m.state)
	}
	if m.totalIssues != 3 {
		t.Errorf("expected 3 issues, got %d", m.totalIssues)
	}
}

func TestModel_IssueProgress(t *testing.T) {
	m := New()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(FixStartedMsg{TotalIssues: 2})
	m.Update(IssueStartedMsg{Index: 0, Total: 2, Action: "coverage", File: "foo.go"})

	if m.currentIndex != 0 {
		t.Errorf("expected index 0, got %d", m.currentIndex)
	}
	if m.currentAction != "coverage" {
		t.Errorf("expected coverage, got %s", m.currentAction)
	}
}

func TestModel_StreamChunk(t *testing.T) {
	m := New()
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(StreamStartedMsg{Label: "fix"})
	m.Update(StreamChunkMsg{Chunk: "package"})
	m.Update(StreamChunkMsg{Chunk: " foo"})
	m.Update(StreamFinishedMsg{})

	if m.streaming {
		t.Error("expected streaming to be false after finish")
	}
	if m.streamBuf.String() != "package foo" {
		t.Errorf("expected 'package foo', got %q", m.streamBuf.String())
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		pct float64
	}{
		{0},
		{50},
		{100},
	}
	for _, tt := range tests {
		got := progressBar(tt.pct, 30)
		if !strings.Contains(got, "]") {
			t.Errorf("expected bracket in %q", got)
		}
		if tt.pct > 0 && tt.pct < 100 && !strings.Contains(got, "█") {
			t.Errorf("progressBar(%v) should have filled chars, got %q", tt.pct, got)
		}
	}
}

func TestIsTerminal(t *testing.T) {
	if IsTerminal() {
		t.Log("running on a real terminal")
	}
}
