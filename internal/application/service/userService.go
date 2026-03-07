package service

import (
	"errors"
	"strings"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.UserUseCase = (*UserService)(nil)
var _ port.CredentialVerifier = (*UserService)(nil)

type UserService struct {
	userRepository     port.UserRepository
	postRepository     port.PostRepository
	commentRepository  port.CommentRepository
	reactionRepository port.ReactionRepository
	passwordHasher     port.PasswordHasher
}

func NewUserService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, passwordHasher port.PasswordHasher) *UserService {
	return &UserService{
		userRepository:     userRepository,
		postRepository:     postRepository,
		commentRepository:  commentRepository,
		reactionRepository: reactionRepository,
		passwordHasher:     passwordHasher,
	}
}

func (s *UserService) SignUp(username, password string) (string, error) {
	// 회원가입 로직 구현
	// duplicate username check
	if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
		return "", customError.ErrInvalidInput
	}
	existingUser, err := s.userRepository.SelectUserByUsername(username)
	if err != nil {
		return "", customError.WrapRepository("select user by username for signup", err)
	}
	if existingUser != nil {
		return "", customError.ErrUserAlreadyExists
	}

	hashedPassword, err := s.passwordHasher.Hash(password)
	if err != nil {
		return "", customError.Wrap(customError.ErrInternalServerError, "hash password for signup", err)
	}
	newUser := entity.NewUser(username, hashedPassword)

	_, err = s.userRepository.Save(newUser)
	if err != nil {
		if errors.Is(err, customError.ErrUserAlreadyExists) {
			return "", customError.ErrUserAlreadyExists
		}
		return "", customError.WrapRepository("save user for signup", err)
	}
	return "ok", nil
}

func (s *UserService) DeleteMe(userID int64, password string) error {
	existingUser, err := s.userRepository.SelectUserByID(userID)
	if err != nil {
		return customError.WrapRepository("select user by id for delete me", err)
	}
	if existingUser == nil {
		return customError.ErrUserNotFound
	}
	matched, err := s.passwordHasher.Matches(existingUser.Password, password)
	if err != nil {
		return customError.Wrap(customError.ErrInternalServerError, "compare password for delete me", err)
	}
	if !matched {
		return customError.ErrInvalidCredential
	}
	hasPosts, err := s.postRepository.ExistsByAuthor(existingUser.ID)
	if err != nil {
		return customError.WrapRepository("check posts by author for delete me", err)
	}
	if hasPosts {
		return customError.ErrUserDeletionBlocked
	}
	hasComments, err := s.commentRepository.ExistsByAuthor(existingUser.ID)
	if err != nil {
		return customError.WrapRepository("check comments by author for delete me", err)
	}
	if hasComments {
		return customError.ErrUserDeletionBlocked
	}
	hasReactions, err := s.reactionRepository.ExistsByUser(existingUser.ID)
	if err != nil {
		return customError.WrapRepository("check reactions by user for delete me", err)
	}
	if hasReactions {
		return customError.ErrUserDeletionBlocked
	}

	err = s.userRepository.Delete(existingUser.ID)
	if err != nil {
		return customError.WrapRepository("delete user for delete me", err)
	}
	return nil
}

func (s *UserService) VerifyCredentials(username, password string) (int64, error) {
	existingUser, err := s.userRepository.SelectUserByUsername(username)
	if err != nil {
		return 0, customError.WrapRepository("select user by username for verify credentials", err)
	}
	if existingUser == nil {
		return 0, customError.ErrInvalidCredential
	}

	matched, err := s.passwordHasher.Matches(existingUser.Password, password)
	if err != nil {
		return 0, customError.Wrap(customError.ErrInternalServerError, "compare password for verify credentials", err)
	}
	if !matched {
		return 0, customError.ErrInvalidCredential
	}
	return existingUser.ID, nil
}
