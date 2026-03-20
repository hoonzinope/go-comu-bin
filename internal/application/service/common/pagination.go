package common

import customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"

const MaxPageLimit = 1000

func RequirePositiveLimit(limit int) error {
	if limit < 1 || limit > MaxPageLimit {
		return customerror.ErrInvalidInput
	}
	return nil
}

func CursorFetchLimit(limit int) (int, error) {
	if err := RequirePositiveLimit(limit); err != nil {
		return 0, err
	}
	return limit + 1, nil
}
