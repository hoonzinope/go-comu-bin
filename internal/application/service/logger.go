package service

import "github.com/hoonzinope/go-comu-bin/internal/application/port"

type noopLogger struct{}

func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

func resolveLogger(loggers []port.Logger) port.Logger {
	if len(loggers) > 0 && loggers[0] != nil {
		return loggers[0]
	}
	return noopLogger{}
}
