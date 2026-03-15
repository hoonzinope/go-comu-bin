package service

import (
	"context"
	"errors"

	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func commentParentUUIDsByID(ctx context.Context, commentRepository port.CommentRepository, comments []*entity.Comment) (map[int64]string, error) {
	parentIDs := make([]int64, 0, len(comments))
	seen := make(map[int64]struct{}, len(comments))
	for _, comment := range comments {
		if comment.ParentID == nil {
			continue
		}
		if _, ok := seen[*comment.ParentID]; ok {
			continue
		}
		seen[*comment.ParentID] = struct{}{}
		parentIDs = append(parentIDs, *comment.ParentID)
	}
	if len(parentIDs) == 0 {
		return map[int64]string{}, nil
	}
	parentUUIDs, err := commentRepository.SelectCommentUUIDsByIDsIncludingDeleted(ctx, parentIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select comment uuids by ids including deleted", err)
	}
	return parentUUIDs, nil
}

func commentModelFromEntity(comment *entity.Comment, postUUID string, authorUUIDs map[int64]string, parentUUIDs map[int64]string) (*model.Comment, error) {
	authorUUID, ok := authorUUIDs[comment.AuthorID]
	if !ok {
		return nil, customerror.WrapRepository("select users by ids including deleted", errors.New("comment author not found"))
	}
	out := mapper.CommentFromEntity(comment)
	out.AuthorUUID = authorUUID
	out.PostUUID = postUUID
	if comment.ParentID != nil {
		if parentUUID, ok := parentUUIDs[*comment.ParentID]; ok {
			out.ParentUUID = &parentUUID
		}
	}
	return &out, nil
}
