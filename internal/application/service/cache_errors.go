package service

import (
	"errors"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func normalizeCacheLoadError(op string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, customError.ErrCacheFailure):
		return err
	case errors.Is(err, customError.ErrRepositoryFailure):
		return err
	case errors.Is(err, customError.ErrTokenFailure):
		return err
	case errors.Is(err, customError.ErrUserAlreadyExists):
		return err
	case errors.Is(err, customError.ErrUserNotFound):
		return err
	case errors.Is(err, customError.ErrBoardNotFound):
		return err
	case errors.Is(err, customError.ErrPostNotFound):
		return err
	case errors.Is(err, customError.ErrCommentNotFound):
		return err
	case errors.Is(err, customError.ErrReactionNotFound):
		return err
	case errors.Is(err, customError.ErrInvalidCredential):
		return err
	case errors.Is(err, customError.ErrInvalidInput):
		return err
	case errors.Is(err, customError.ErrUnauthorized):
		return err
	case errors.Is(err, customError.ErrMissingAuthHeader):
		return err
	case errors.Is(err, customError.ErrInvalidToken):
		return err
	case errors.Is(err, customError.ErrForbidden):
		return err
	default:
		return customError.WrapCache(op, err)
	}
}
