package service

import (
	"errors"
	"fmt"
	"sort"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

func userUUIDByID(userRepository port.UserRepository, userID int64) (string, error) {
	usersByID, err := userUUIDsByIDs(userRepository, []int64{userID})
	if err != nil {
		return "", err
	}
	userUUID, ok := usersByID[userID]
	if !ok {
		return "", customError.WrapRepository("select user by id including deleted", fmt.Errorf("user %d: %w", userID, errors.New("not found")))
	}
	return userUUID, nil
}

func userUUIDsByIDs(userRepository port.UserRepository, ids []int64) (map[int64]string, error) {
	uniqueIDs := uniqueInt64s(ids)
	usersByID, err := userRepository.SelectUsersByIDsIncludingDeleted(uniqueIDs)
	if err != nil {
		return nil, customError.WrapRepository("select users by ids including deleted", err)
	}
	out := make(map[int64]string, len(usersByID))
	for _, id := range uniqueIDs {
		user, ok := usersByID[id]
		if !ok || user == nil {
			return nil, customError.WrapRepository("select users by ids including deleted", fmt.Errorf("user %d: %w", id, errors.New("not found")))
		}
		out[id] = user.UUID
	}
	return out, nil
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
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	return out
}

func userUUIDsForPosts(userRepository port.UserRepository, posts []*entity.Post) (map[int64]string, error) {
	ids := make([]int64, 0, len(posts))
	for _, post := range posts {
		ids = append(ids, post.AuthorID)
	}
	return userUUIDsByIDs(userRepository, ids)
}

func userUUIDsForComments(userRepository port.UserRepository, comments []*entity.Comment) (map[int64]string, error) {
	ids := make([]int64, 0, len(comments))
	for _, comment := range comments {
		ids = append(ids, comment.AuthorID)
	}
	return userUUIDsByIDs(userRepository, ids)
}

func userUUIDsForReactions(userRepository port.UserRepository, reactions []*entity.Reaction) (map[int64]string, error) {
	ids := make([]int64, 0, len(reactions))
	for _, reaction := range reactions {
		ids = append(ids, reaction.UserID)
	}
	return userUUIDsByIDs(userRepository, ids)
}
