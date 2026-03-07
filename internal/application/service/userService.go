package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.UserUseCase = (*UserService)(nil)
var _ port.CredentialVerifier = (*UserService)(nil)

type UserService struct {
	userRepository port.UserRepository
}

func NewUserService(userRepository port.UserRepository) *UserService {
	return &UserService{
		userRepository: userRepository,
	}
}

func (s *UserService) SignUp(username, password string) (string, error) {
	// 회원가입 로직 구현
	// duplicate username check
	existingUser, err := s.userRepository.SelectUserByUsername(username)
	if err != nil {
		return "", customError.ErrInternalServerError
	}
	if existingUser != nil {
		return "", customError.ErrUserAlreadyExists
	}

	newUser := entity.NewUser(username, password)

	_, err = s.userRepository.Save(newUser)
	if err != nil {
		return "", customError.ErrInternalServerError
	}
	return "ok", nil
}

func (s *UserService) DeleteMe(userID int64, password string) error {
	existingUser, err := s.userRepository.SelectUserByID(userID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	if existingUser == nil {
		return customError.ErrUserNotFound
	}
	if existingUser.Password != password {
		return customError.ErrInvalidCredential
	}

	err = s.userRepository.Delete(existingUser.ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}

func (s *UserService) VerifyCredentials(username, password string) (int64, error) {
	existingUser, err := s.userRepository.SelectUserByUsername(username)
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
