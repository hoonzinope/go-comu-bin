package post

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

const searchCursorPrefix = "v1:"

func decodeSearchCursor(cursor string) (*port.PostSearchCursor, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, customerror.ErrInvalidInput
	}
	decoded := string(raw)
	if !strings.HasPrefix(decoded, searchCursorPrefix) {
		return nil, customerror.ErrInvalidInput
	}
	parts := strings.Split(strings.TrimPrefix(decoded, searchCursorPrefix), ":")
	if len(parts) != 2 {
		return nil, customerror.ErrInvalidInput
	}
	scoreBits, err := strconv.ParseUint(parts[0], 16, 64)
	if err != nil {
		return nil, customerror.ErrInvalidInput
	}
	postID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || postID < 0 {
		return nil, customerror.ErrInvalidInput
	}
	return &port.PostSearchCursor{Score: math.Float64frombits(scoreBits), PostID: postID}, nil
}

func encodeSearchCursor(score float64, postID int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s%x:%d", searchCursorPrefix, math.Float64bits(score), postID)))
}
