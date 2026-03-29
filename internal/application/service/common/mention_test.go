package common

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestTruncateNotificationSnapshot_IsUTF8Safe(t *testing.T) {
	got := TruncateNotificationSnapshot("café au lait", 4)
	assert.True(t, utf8.ValidString(got))
	assert.Equal(t, "caf", got)
}
