package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/hoonzinope/go-comu-bin/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

func newAppLogger(stdout io.Writer, cfg *config.Config) (*slog.Logger, io.Closer, error) {
	if stdout == nil {
		stdout = io.Discard
	}
	rotatingWriter := &lumberjack.Logger{
		Filename:   cfg.Logging.FilePath,
		MaxSize:    cfg.Logging.MaxSizeMB,
		MaxBackups: cfg.Logging.MaxBackups,
		MaxAge:     cfg.Logging.MaxAgeDays,
		Compress:   cfg.Logging.Compress,
		LocalTime:  cfg.Logging.LocalTime,
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Logging.FilePath), 0o755); err != nil {
		return nil, nil, err
	}
	writer := io.MultiWriter(stdout, rotatingWriter)
	logger := slog.New(slog.NewJSONHandler(writer, nil))
	return logger, rotatingWriter, nil
}
