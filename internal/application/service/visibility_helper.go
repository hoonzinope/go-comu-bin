package service

import (
	"context"
	"errors"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type boardByIDReader interface {
	SelectBoardByID(ctx context.Context, id int64) (*entity.Board, error)
}

type postByIDReader interface {
	SelectPostByID(ctx context.Context, id int64) (*entity.Post, error)
}

type commentByIDReader interface {
	SelectCommentByID(ctx context.Context, id int64) (*entity.Comment, error)
}

func ensureBoardVisibleForUser(ctx context.Context, boardRepository boardByIDReader, user *entity.User, boardID int64, concealedErr error, action string) error {
	board, err := boardRepository.SelectBoardByID(ctx, boardID)
	if err != nil {
		return customerror.WrapRepository("select board by id for "+action, err)
	}
	if err := policy.EnsureBoardVisible(board, user); err != nil {
		if concealedErr != nil && errors.Is(err, customerror.ErrBoardNotFound) {
			return concealedErr
		}
		return err
	}
	return nil
}

func ensurePostVisibleForUser(ctx context.Context, postRepository postByIDReader, boardRepository boardByIDReader, user *entity.User, postID int64, concealedErr error, action string) (*entity.Post, error) {
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
	if err := ensureBoardVisibleForUser(ctx, boardRepository, user, post.BoardID, concealedErr, action); err != nil {
		return nil, err
	}
	return post, nil
}

func ensureCommentTargetVisibleForUser(ctx context.Context, commentRepository commentByIDReader, postRepository postByIDReader, boardRepository boardByIDReader, user *entity.User, commentID int64, concealedErr error, action string) (*entity.Comment, *entity.Post, error) {
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
	post, err := ensurePostVisibleForUser(ctx, postRepository, boardRepository, user, comment.PostID, concealedErr, action)
	if err != nil {
		return nil, nil, err
	}
	return comment, post, nil
}
