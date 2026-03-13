package service

import (
	"io"
	"log/slog"
)

func resolveLogger(loggers []*slog.Logger) *slog.Logger {
	if len(loggers) > 0 && loggers[0] != nil {
		return loggers[0]
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
