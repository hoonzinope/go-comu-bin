package service

import customError "github.com/hoonzinope/go-comu-bin/internal/customError"

func requirePositiveLimit(limit int) error {
	if limit < 1 {
		return customError.ErrInvalidInput
	}
	return nil
}
