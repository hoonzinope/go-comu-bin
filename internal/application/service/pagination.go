package service

import (
	"math"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func requirePositiveLimit(limit int) error {
	if limit < 1 {
		return customError.ErrInvalidInput
	}
	return nil
}

func cursorFetchLimit(limit int) (int, error) {
	if limit > math.MaxInt-1 {
		return 0, customError.ErrInvalidInput
	}
	return limit + 1, nil
}
