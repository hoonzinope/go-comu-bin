package customerror

import (
	"errors"
	"fmt"
)

var (
	// Public/common
	ErrInternalServerError = errors.New("internal server error")
	ErrForbidden           = errors.New("forbidden")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrTooManyRequests     = errors.New("too many requests")
	ErrInvalidInput        = errors.New("invalid input")
	ErrNotFound            = errors.New("not found")
	ErrMethodNotAllowed    = errors.New("method not allowed")
	ErrInvalidCredential   = errors.New("invalid credential")
	ErrMissingAuthHeader   = errors.New("missing Authorization header")
	ErrInvalidToken        = errors.New("invalid token")
	ErrUserSuspended       = errors.New("user suspended")

	// Public/resource
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrUserNotFound        = errors.New("user not found")
	ErrBoardNotFound       = errors.New("board not found")
	ErrBoardNotEmpty       = errors.New("board not empty")
	ErrPostNotFound        = errors.New("post not found")
	ErrTagNotFound         = errors.New("tag not found")
	ErrAttachmentNotFound  = errors.New("attachment not found")
	ErrCommentNotFound     = errors.New("comment not found")
	ErrReactionNotFound    = errors.New("reaction not found")
	ErrReportNotFound      = errors.New("report not found")
	ErrReportAlreadyExists = errors.New("report already exists")

	// Internal categories
	ErrRepositoryFailure = errors.New("repository failure")
	ErrCacheFailure      = errors.New("cache failure")
	ErrTokenFailure      = errors.New("token failure")
)

func Mark(kind error, op string) error {
	return fmt.Errorf("%s: %w", op, kind)
}

func Wrap(kind error, op string, err error) error {
	if err == nil {
		return Mark(kind, op)
	}
	return fmt.Errorf("%s: %w: %w", op, kind, err)
}

func WrapRepository(op string, err error) error {
	return Wrap(ErrRepositoryFailure, op, err)
}

func WrapCache(op string, err error) error {
	return Wrap(ErrCacheFailure, op, err)
}

func WrapToken(op string, err error) error {
	return Wrap(ErrTokenFailure, op, err)
}

func Public(err error) error {
	switch {
	case errors.Is(err, ErrUserAlreadyExists):
		return ErrUserAlreadyExists
	case errors.Is(err, ErrUserNotFound):
		return ErrUserNotFound
	case errors.Is(err, ErrBoardNotFound):
		return ErrBoardNotFound
	case errors.Is(err, ErrBoardNotEmpty):
		return ErrBoardNotEmpty
	case errors.Is(err, ErrPostNotFound):
		return ErrPostNotFound
	case errors.Is(err, ErrTagNotFound):
		return ErrTagNotFound
	case errors.Is(err, ErrAttachmentNotFound):
		return ErrAttachmentNotFound
	case errors.Is(err, ErrCommentNotFound):
		return ErrCommentNotFound
	case errors.Is(err, ErrReactionNotFound):
		return ErrReactionNotFound
	case errors.Is(err, ErrReportNotFound):
		return ErrReportNotFound
	case errors.Is(err, ErrReportAlreadyExists):
		return ErrReportAlreadyExists
	case errors.Is(err, ErrInvalidCredential):
		return ErrInvalidCredential
	case errors.Is(err, ErrUserSuspended):
		return ErrUserSuspended
	case errors.Is(err, ErrInvalidInput):
		return ErrInvalidInput
	case errors.Is(err, ErrNotFound):
		return ErrNotFound
	case errors.Is(err, ErrMethodNotAllowed):
		return ErrMethodNotAllowed
	case errors.Is(err, ErrUnauthorized):
		return ErrUnauthorized
	case errors.Is(err, ErrTooManyRequests):
		return ErrTooManyRequests
	case errors.Is(err, ErrMissingAuthHeader):
		return ErrMissingAuthHeader
	case errors.Is(err, ErrInvalidToken):
		return ErrInvalidToken
	case errors.Is(err, ErrForbidden):
		return ErrForbidden
	default:
		return ErrInternalServerError
	}
}
