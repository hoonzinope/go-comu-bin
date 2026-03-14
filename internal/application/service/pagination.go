package service

import customError "github.com/hoonzinope/go-comu-bin/internal/customError"

const maxPageLimit = 1000

func requirePositiveLimit(limit int) error {
	if limit < 1 || limit > maxPageLimit {
		return customError.ErrInvalidInput
	}
	return nil
}

func cursorFetchLimit(limit int) (int, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return 0, err
	}
	return limit + 1, nil
}
