package service

import (
	"errors"
	"fmt"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
)

func userUUIDByID(userRepository port.UserRepository, userID int64) (string, error) {
	user, err := userRepository.SelectUserByIDIncludingDeleted(userID)
	if err != nil {
		return "", customError.WrapRepository("select user by id including deleted", err)
	}
	if user == nil {
		return "", customError.WrapRepository("select user by id including deleted", fmt.Errorf("user %d: %w", userID, errors.New("not found")))
	}
	return user.UUID, nil
}
