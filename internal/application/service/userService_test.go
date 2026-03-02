package service

import (
	"errors"
	"testing"

	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func TestUserService_SignUp_Success(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)

	result, err := svc.SignUp("alice", "pw")
	if err != nil {
		t.Fatalf("SignUp returned error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestUserService_SignUp_Duplicate(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)
	_, _ = svc.SignUp("alice", "pw")

	_, err := svc.SignUp("alice", "pw2")
	if !errors.Is(err, customError.ErrUserAlreadyExists) {
		t.Fatalf("expected ErrUserAlreadyExists, got: %v", err)
	}
}

func TestUserService_Quit_InvalidCredential(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)
	_, _ = svc.SignUp("alice", "pw")

	err := svc.Quit("alice", "wrong")
	if !errors.Is(err, customError.ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got: %v", err)
	}
}

func TestUserService_Quit_Success(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)
	_, _ = svc.SignUp("alice", "pw")

	err := svc.Quit("alice", "pw")
	if err != nil {
		t.Fatalf("Quit returned error: %v", err)
	}
}

func TestUserService_Quit_UserNotFound(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)

	err := svc.Quit("nope", "pw")
	if !errors.Is(err, customError.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestUserService_Login_UserNotFound(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)

	_, err := svc.Login("nope", "pw")
	if !errors.Is(err, customError.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestUserService_Login_WrongPassword(t *testing.T) {
	repository := newTestRepository()
	svc := NewUserService(repository)
	_, _ = svc.SignUp("alice", "pw")

	_, err := svc.Login("alice", "wrong")
	if !errors.Is(err, customError.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got: %v", err)
	}
}
