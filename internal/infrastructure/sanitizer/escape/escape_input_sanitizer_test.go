package escape

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInputSanitizer_Sanitize(t *testing.T) {
	sanitizer := NewInputSanitizer()
	out, err := sanitizer.Sanitize(context.Background(), `<script>alert(1)</script>`)
	require.NoError(t, err)
	assert.Equal(t, `&lt;script&gt;alert(1)&lt;/script&gt;`, out)
}
