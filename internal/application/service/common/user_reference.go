package common

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func UserUUIDsByIDs(ctx context.Context, userRepository port.UserRepository, ids []int64) (map[int64]string, error) {
	uniqueIDs := uniqueInt64s(ids)
	usersByID, err := userRepository.SelectUsersByIDsIncludingDeleted(ctx, uniqueIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select users by ids including deleted", err)
	}
	out := make(map[int64]string, len(usersByID))
	for _, id := range uniqueIDs {
		user, ok := usersByID[id]
		if !ok || user == nil {
			return nil, customerror.WrapRepository("select users by ids including deleted", fmt.Errorf("user %d: %w", id, errors.New("not found")))
		}
		out[id] = user.UUID
	}
	return out, nil
}

func UserUUIDsForPosts(ctx context.Context, userRepository port.UserRepository, posts []*entity.Post) (map[int64]string, error) {
	ids := make([]int64, 0, len(posts))
	for _, post := range posts {
		ids = append(ids, post.AuthorID)
	}
	return UserUUIDsByIDs(ctx, userRepository, ids)
}

func UserUUIDsForComments(ctx context.Context, userRepository port.UserRepository, comments []*entity.Comment) (map[int64]string, error) {
	ids := make([]int64, 0, len(comments))
	for _, comment := range comments {
		ids = append(ids, comment.AuthorID)
	}
	return UserUUIDsByIDs(ctx, userRepository, ids)
}

func UserUUIDsForReactions(ctx context.Context, userRepository port.UserRepository, reactions []*entity.Reaction) (map[int64]string, error) {
	ids := make([]int64, 0, len(reactions))
	for _, reaction := range reactions {
		ids = append(ids, reaction.UserID)
	}
	return UserUUIDsByIDs(ctx, userRepository, ids)
}

func uniqueInt64s(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
