package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ application.UserUseCase = (*UserService)(nil)

type UserService struct {
	repository application.Repository
}

func NewUserService(repository application.Repository) *UserService {
	return &UserService{
		repository: repository,
	}
}

func (s *UserService) SignUp(username, password string) (string, error) {
	// 회원가입 로직 구현
	// duplicate username check
	existingUser, err := s.repository.UserRepository.SelectUserByUsername(username)
	if err != nil {
		return "", customError.ErrInternalServerError
	}
	if existingUser != nil {
		return "", customError.ErrUserAlreadyExists
	}

	newUser := entity.NewUser(username, password)

	_, err = s.repository.UserRepository.Save(newUser)
	if err != nil {
		return "", customError.ErrInternalServerError
	}
	return "ok", nil
}

func (s *UserService) Quit(username, password string) error {
	// 회원탈퇴 로직 구현
	// user 존재 여부 확인
	existingUser, err := s.repository.UserRepository.SelectUserByUsername(username)
	if err != nil {
		return customError.ErrInternalServerError
	}
	if existingUser == nil {
		return customError.ErrUserNotFound
	}
	if existingUser.Password != password {
		return customError.ErrInvalidCredential
	}

	err = s.repository.UserRepository.Delete(existingUser.ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}

func (s *UserService) Login(username, password string) (int64, error) {
	// 로그인 로직 구현
	// user 존재 여부 확인
	existingUser, err := s.repository.UserRepository.SelectUserByUsername(username)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	if existingUser == nil {
		return 0, customError.ErrUserNotFound
	}

	// password check (실제 구현에서는 해시된 비밀번호 비교 필요)
	if existingUser.Password != password {
		return 0, customError.ErrUserNotFound
	}
	return existingUser.ID, nil
}

func (s *UserService) Logout(username string) error {
	// 로그아웃 로직 구현
	// 실제 구현에서는 세션 관리 필요
	return nil
}
