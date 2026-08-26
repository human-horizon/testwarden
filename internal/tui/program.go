package tui

import (
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/HumanHorizon/testwarden/internal/report"
)

// IsTerminal reports whether stdout is a real terminal.
func IsTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Options control the TUI.
type Options struct {
	Interactive bool
	Output      io.Writer
}

// Program wraps a bubbletea.Program for easy invocation.
type Program struct {
	model *Model
	prog  *tea.Program
	opts  Options
}

// NewProgram creates a Program from options.
func NewProgram(opts Options) *Program {
	return &Program{model: New(), opts: opts}
}

// Send pushes a message into the program.
func (p *Program) Send(msg tea.Msg) {
	if p.model != nil && p.prog != nil {
		p.prog.Send(msg)
	}
}

// Run starts the TUI loop. Returns the final Model.
func (p *Program) Run() (*Model, error) {
	if !p.opts.Interactive {
		return p.model, nil
	}
	p.prog = tea.NewProgram(p.model, tea.WithOutput(p.opts.Output))
	finalModel, err := p.prog.Run()
	if err != nil {
		return nil, err
	}
	if m, ok := finalModel.(*Model); ok {
		return m, nil
	}
	return p.model, nil
}

// Model exposes the underlying model.
func (p *Program) Model() *Model { return p.model }

// Results returns the final results from the model.
func (p *Program) Results() []*report.Result {
	if p.model == nil {
		return nil
	}
	return p.model.results
}
