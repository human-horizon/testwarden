package runner

import (
	"fmt"
	"io"
	"os"

	"github.com/HumanHorizon/testwarden/internal/report"
	"github.com/HumanHorizon/testwarden/internal/tui"
)

// Progress reports progress to the user. Implementations: TUI, plain, nop.
type Progress interface {
	AnalyseStarted()
	FixStarted(totalIssues int)
	IssueStarted(index, total int, action, file string)
	StreamStarted(label string)
	StreamChunk(chunk string)
	StreamFinished()
	Done(results []*report.Result)
	Error(err error)
	Wait() error
}

// nopProgress is a no-op progress reporter.
type nopProgress struct{}

func (nopProgress) AnalyseStarted()                       {}
func (nopProgress) FixStarted(int)                        {}
func (nopProgress) IssueStarted(int, int, string, string) {}
func (nopProgress) StreamStarted(string)                  {}
func (nopProgress) StreamChunk(string)                    {}
func (nopProgress) StreamFinished()                       {}
func (nopProgress) Done([]*report.Result)                 {}
func (nopProgress) Error(error)                           {}
func (nopProgress) Wait() error                           { return nil }

// tuiProgress wraps a bubbletea program.
type tuiProgress struct {
	prog *tui.Program
}

func newTUIProgress() *tuiProgress {
	return &tuiProgress{prog: tui.NewProgram(tui.Options{
		Interactive: tui.IsTerminal(),
		Output:      os.Stdout,
	})}
}

func (p *tuiProgress) AnalyseStarted() { p.prog.Send(tui.AnalyseStartedMsg{}) }
func (p *tuiProgress) FixStarted(totalIssues int) {
	p.prog.Send(tui.FixStartedMsg{TotalIssues: totalIssues})
}
func (p *tuiProgress) IssueStarted(index, total int, action, file string) {
	p.prog.Send(tui.IssueStartedMsg{Index: index, Total: total, Action: action, File: file})
}
func (p *tuiProgress) StreamStarted(label string) { p.prog.Send(tui.StreamStartedMsg{Label: label}) }
func (p *tuiProgress) StreamChunk(chunk string)   { p.prog.Send(tui.StreamChunkMsg{Chunk: chunk}) }
func (p *tuiProgress) StreamFinished()            { p.prog.Send(tui.StreamFinishedMsg{}) }
func (p *tuiProgress) Done(results []*report.Result) {
	converted := make([]any, len(results))
	for i, r := range results {
		converted[i] = r
	}
	p.prog.Send(tui.DoneMsg{Results: results})
	_ = converted
}
func (p *tuiProgress) Error(err error) { p.prog.Send(tui.ErrorMsg{Err: err}) }
func (p *tuiProgress) Wait() error     { _, err := p.prog.Run(); return err }

// plainProgress writes to a writer in plain text.
type plainProgress struct {
	out io.Writer
}

func (p *plainProgress) AnalyseStarted() { fmt.Fprintln(p.out, "→ Analysing project...") }
func (p *plainProgress) FixStarted(totalIssues int) {
	fmt.Fprintf(p.out, "→ Found %d issue(s) to fix\n", totalIssues)
}
func (p *plainProgress) IssueStarted(i, total int, action, file string) {
	fmt.Fprintf(p.out, "[%d/%d] %s: %s\n", i+1, total, action, file)
}
func (p *plainProgress) StreamStarted(string) {}
func (p *plainProgress) StreamChunk(chunk string) {
	if f, ok := p.out.(*os.File); ok && isTerminal(f) {
		fmt.Fprint(p.out, chunk)
	}
}
func (p *plainProgress) StreamFinished()       { fmt.Fprintln(p.out) }
func (p *plainProgress) Done([]*report.Result) {}
func (p *plainProgress) Error(err error)       { fmt.Fprintf(p.out, "✗ Error: %v\n", err) }
func (p *plainProgress) Wait() error           { return nil }

// newProgress creates the right Progress for the environment.
func newProgress(opts Options) Progress {
	if opts.NoTUI || !tui.IsTerminal() {
		return &plainProgress{out: opts.Out}
	}
	return newTUIProgress()
}
