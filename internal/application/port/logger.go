package port

type Logger interface {
	Warn(msg string, args ...any)
}
