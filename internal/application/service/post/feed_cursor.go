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

const feedCursorPrefix = "v1:"

func decodeFeedCursor(sortValue string, windowValue string, cursor string) (*port.PostFeedCursor, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, customerror.ErrInvalidInput
	}
	decoded := string(raw)
	if !strings.HasPrefix(decoded, feedCursorPrefix) {
		return nil, customerror.ErrInvalidInput
	}
	parts := strings.Split(strings.TrimPrefix(decoded, feedCursorPrefix), ":")
	if len(parts) != 5 {
		return nil, customerror.ErrInvalidInput
	}
	sortBy := port.PostFeedSort(parts[0])
	if string(sortBy) != sortValue {
		return nil, customerror.ErrInvalidInput
	}
	window := port.PostRankingWindow(parts[1])
	if string(window) != windowValue {
		return nil, customerror.ErrInvalidInput
	}
	scoreBits, err := strconv.ParseUint(parts[2], 16, 64)
	if err != nil {
		return nil, customerror.ErrInvalidInput
	}
	publishedAtUnixNano, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, customerror.ErrInvalidInput
	}
	postID, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil || postID < 0 {
		return nil, customerror.ErrInvalidInput
	}
	return &port.PostFeedCursor{
		Sort:                sortBy,
		Window:              window,
		Score:               math.Float64frombits(scoreBits),
		PublishedAtUnixNano: publishedAtUnixNano,
		PostID:              postID,
	}, nil
}

func encodeFeedCursor(sortBy port.PostFeedSort, window port.PostRankingWindow, score float64, publishedAtUnixNano, postID int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s%s:%s:%x:%d:%d", feedCursorPrefix, sortBy, window, math.Float64bits(score), publishedAtUnixNano, postID)))
}
