package policy

import (
	"context"
	"errors"

	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type BoardByIDReader interface {
	SelectBoardByID(ctx context.Context, id int64) (*entity.Board, error)
}

type PostByIDReader interface {
	SelectPostByID(ctx context.Context, id int64) (*entity.Post, error)
}

type CommentByIDReader interface {
	SelectCommentByID(ctx context.Context, id int64) (*entity.Comment, error)
}

func EnsureBoardVisibleForUser(ctx context.Context, boardRepository BoardByIDReader, user *entity.User, boardID int64, concealedErr error, action string) error {
	board, err := boardRepository.SelectBoardByID(ctx, boardID)
	if err != nil {
		return customerror.WrapRepository("select board by id for "+action, err)
	}
	if err := EnsureBoardVisible(board, user); err != nil {
		if concealedErr != nil && errors.Is(err, customerror.ErrBoardNotFound) {
			return concealedErr
		}
		return err
	}
	return nil
}

func EnsurePostVisibleForUser(ctx context.Context, postRepository PostByIDReader, boardRepository BoardByIDReader, user *entity.User, postID int64, concealedErr error, action string) (*entity.Post, error) {
	post, err := postRepository.SelectPostByID(ctx, postID)
	if err != nil {
		return nil, customerror.WrapRepository("select post by id for "+action, err)
	}
	if post == nil {
		if concealedErr != nil {
			return nil, concealedErr
		}
		return nil, customerror.ErrPostNotFound
	}
	if err := EnsureBoardVisibleForUser(ctx, boardRepository, user, post.BoardID, concealedErr, action); err != nil {
		return nil, err
	}
	return post, nil
}

func EnsureCommentTargetVisibleForUser(ctx context.Context, commentRepository CommentByIDReader, postRepository PostByIDReader, boardRepository BoardByIDReader, user *entity.User, commentID int64, concealedErr error, action string) (*entity.Comment, *entity.Post, error) {
	comment, err := commentRepository.SelectCommentByID(ctx, commentID)
	if err != nil {
		return nil, nil, customerror.WrapRepository("select comment by id for "+action, err)
	}
	if comment == nil {
		if concealedErr != nil {
			return nil, nil, concealedErr
		}
		return nil, nil, customerror.ErrCommentNotFound
	}
	post, err := EnsurePostVisibleForUser(ctx, postRepository, boardRepository, user, comment.PostID, concealedErr, action)
	if err != nil {
		return nil, nil, err
	}
	return comment, post, nil
}
