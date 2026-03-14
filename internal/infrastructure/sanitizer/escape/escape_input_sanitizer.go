package escape

import (
	"context"
	"html"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.InputSanitizer = (*InputSanitizer)(nil)

type InputSanitizer struct{}

func NewInputSanitizer() *InputSanitizer {
	return &InputSanitizer{}
}

func (s *InputSanitizer) Sanitize(ctx context.Context, input string) (string, error) {
	_ = ctx
	return html.EscapeString(input), nil
}
