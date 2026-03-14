package service

import customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"

const maxPageLimit = 1000

func requirePositiveLimit(limit int) error {
	if limit < 1 || limit > maxPageLimit {
		return customerror.ErrInvalidInput
	}
	return nil
}

func cursorFetchLimit(limit int) (int, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return 0, err
	}
	return limit + 1, nil
}
