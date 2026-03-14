package port

import "context"

type InputSanitizer interface {
	Sanitize(ctx context.Context, input string) (string, error)
}
