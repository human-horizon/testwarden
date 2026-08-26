package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGreen  = "\033[32m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// ColorHandler is a slog.Handler that writes colored text output for terminals.
type ColorHandler struct {
	out    io.Writer
	opts   *slog.HandlerOptions
	groups []string
	attrs  []slog.Attr
}

// NewColorHandler creates a new ColorHandler.
func NewColorHandler(out io.Writer, opts *slog.HandlerOptions) *ColorHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &ColorHandler{out: out, opts: opts}
}

// Enabled implements slog.Handler.
func (h *ColorHandler) Enabled(_ context.Context, level slog.Level) bool {
	min := slog.LevelInfo
	if h.opts.Level != nil {
		min = h.opts.Level.Level()
	}
	return level >= min
}

// Handle implements slog.Handler.
func (h *ColorHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder

	// Time (gray)
	b.WriteString(colorGray)
	b.WriteString(r.Time.Format("15:04:05.000"))
	b.WriteString(colorReset)
	b.WriteString(" ")

	// Level (colored)
	levelColor := colorBlue
	switch r.Level {
	case slog.LevelWarn:
		levelColor = colorYellow
	case slog.LevelError:
		levelColor = colorRed
	case slog.LevelDebug:
		levelColor = colorGray
	}
	fmt.Fprintf(&b, "%s%-5s%s ", levelColor+colorBold, strings.ToUpper(r.Level.String()), colorReset)

	// Message (bold)
	b.WriteString(colorBold)
	b.WriteString(r.Message)
	b.WriteString(colorReset)

	// Attributes
	prefix := ""
	for _, a := range h.attrs {
		prefix += " " + colorizeAttr(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		prefix += " " + colorizeAttr(a)
		return true
	})
	if prefix != "" {
		b.WriteString(colorGray)
		b.WriteString(prefix)
		b.WriteString(colorReset)
	}

	b.WriteString("\n")
	_, err := io.WriteString(h.out, b.String())
	return err
}

// WithAttrs implements slog.Handler.
func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ColorHandler{
		out:    h.out,
		opts:   h.opts,
		groups: h.groups,
		attrs:  append(append([]slog.Attr{}, h.attrs...), attrs...),
	}
}

// WithGroup implements slog.Handler.
func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return &ColorHandler{
		out:    h.out,
		opts:   h.opts,
		groups: append(append([]string{}, h.groups...), name),
		attrs:  h.attrs,
	}
}

func colorizeAttr(a slog.Attr) string {
	key := a.Key
	if a.Value.Kind() == slog.KindString {
		return fmt.Sprintf("%s=%q", key, a.Value.String())
	}
	return fmt.Sprintf("%s=%v", key, a.Value.Any())
}
