package common

import (
	"errors"

	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
)

func NormalizeCacheLoadError(op string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, customerror.ErrCacheFailure):
		return err
	case errors.Is(err, customerror.ErrRepositoryFailure):
		return err
	case errors.Is(err, customerror.ErrTokenFailure):
		return err
	case errors.Is(err, customerror.ErrUserAlreadyExists):
		return err
	case errors.Is(err, customerror.ErrUserNotFound):
		return err
	case errors.Is(err, customerror.ErrBoardNotFound):
		return err
	case errors.Is(err, customerror.ErrPostNotFound):
		return err
	case errors.Is(err, customerror.ErrTagNotFound):
		return err
	case errors.Is(err, customerror.ErrCommentNotFound):
		return err
	case errors.Is(err, customerror.ErrReactionNotFound):
		return err
	case errors.Is(err, customerror.ErrInvalidCredential):
		return err
	case errors.Is(err, customerror.ErrInvalidInput):
		return err
	case errors.Is(err, customerror.ErrUnauthorized):
		return err
	case errors.Is(err, customerror.ErrMissingAuthHeader):
		return err
	case errors.Is(err, customerror.ErrInvalidToken):
		return err
	case errors.Is(err, customerror.ErrForbidden):
		return err
	default:
		return customerror.WrapCache(op, err)
	}
}
