package logging

import (
	"log/slog"
	"time"
)

func slogLevelInfo() slog.Level { return slog.LevelInfo }

func newRecord(msg string, attrs ...any) slog.Record {
	r := slog.Record{
		Time:    time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Message: msg,
		Level:   slog.LevelInfo,
	}
	for i := 0; i+1 < len(attrs); i += 2 {
		key, _ := attrs[i].(string)
		r.AddAttrs(slog.Any(key, attrs[i+1]))
	}
	return r
}
