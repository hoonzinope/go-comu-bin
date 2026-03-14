package policy

import (
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

// EnsureBoardVisible applies concealment policy for hidden boards.
// Hidden boards are treated as not found for non-admin users.
func EnsureBoardVisible(board *entity.Board, user *entity.User) error {
	if board == nil {
		return customError.ErrBoardNotFound
	}
	if !board.Hidden || (user != nil && user.IsAdmin()) {
		return nil
	}
	return customError.ErrBoardNotFound
}
