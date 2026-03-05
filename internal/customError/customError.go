package customerror

import "errors"

var (
	// 공통 에러 정의
	ErrInternalServerError = errors.New("internal server error")
	ErrForbidden           = errors.New("forbidden")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrInvalidCredential   = errors.New("invalid credential")
	ErrMissingAuthHeader   = errors.New("missing Authorization header")
	ErrInvalidToken        = errors.New("invalid token")
	// User 관련 에러 정의
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
	ErrBoardNotFound     = errors.New("board not found")
	ErrPostNotFound      = errors.New("post not found")
	ErrCommentNotFound   = errors.New("comment not found")
	ErrReactionNotFound  = errors.New("reaction not found")
	ErrSaveUserFailed    = errors.New("failed to save user")
	ErrDeleteUserFailed  = errors.New("failed to delete user")
)
