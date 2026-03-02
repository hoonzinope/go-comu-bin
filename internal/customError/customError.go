package error

import "errors"

var (
	// 공통 에러 정의
	ErrInternalServerError = errors.New("internal server error")
	// User 관련 에러 정의
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrUserNotFound      = errors.New("user not found")
	ErrSaveUserFailed    = errors.New("failed to save user")
	ErrDeleteUserFailed  = errors.New("failed to delete user")
)
