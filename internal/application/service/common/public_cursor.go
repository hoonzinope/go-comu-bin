package common

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

const cursorPrefix = "v1:"

func DecodeOpaqueCursor(cursor string) (int64, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, customerror.ErrInvalidInput
	}
	decoded := string(raw)
	if !strings.HasPrefix(decoded, cursorPrefix) {
		return 0, customerror.ErrInvalidInput
	}
	lastID, err := strconv.ParseInt(strings.TrimPrefix(decoded, cursorPrefix), 10, 64)
	if err != nil || lastID < 0 {
		return 0, customerror.ErrInvalidInput
	}
	return lastID, nil
}

func EncodeOpaqueCursor(lastID int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s%d", cursorPrefix, lastID)))
}
