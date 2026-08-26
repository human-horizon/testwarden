// Package logging provides structured logging via log/slog with terminal-friendly output.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Level controls log verbosity.
type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

// Options configures the logger.
type Options struct {
	Level  Level
	JSON   bool
	Output io.Writer
	Color  bool
}

// New creates a configured slog.Logger.
func New(opts Options) *slog.Logger {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}

	level := slog.LevelInfo
	switch opts.Level {
	case LevelDebug:
		level = slog.LevelDebug
	case LevelInfo:
		level = slog.LevelInfo
	case LevelWarn:
		level = slog.LevelWarn
	case LevelError:
		level = slog.LevelError
	}

	handlerOpts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if opts.JSON {
		handler = slog.NewJSONHandler(opts.Output, handlerOpts)
	} else if opts.Color {
		handler = NewColorHandler(opts.Output, handlerOpts)
	} else {
		handler = slog.NewTextHandler(opts.Output, handlerOpts)
	}

	return slog.New(handler)
}

// ParseLevel converts a string to Level.
func ParseLevel(s string) Level {
	switch strings.ToLower(s) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// WithLogger returns a copy of the parent context carrying the logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// FromContext extracts a logger from context (or returns the default).
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

type ctxKey struct{}
