package service

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

func filterVisibleComments(comments []*entity.Comment, lastID int64) []*entity.Comment {
	activeChildParentIDs := make(map[int64]struct{})
	for _, comment := range comments {
		if comment.Status == entity.CommentStatusActive && comment.ParentID != nil {
			activeChildParentIDs[*comment.ParentID] = struct{}{}
		}
	}

	filtered := make([]*entity.Comment, 0, len(comments))
	for _, comment := range comments {
		if lastID > 0 && comment.ID >= lastID {
			continue
		}
		if comment.Status == entity.CommentStatusActive {
			filtered = append(filtered, comment)
			continue
		}
		if _, ok := activeChildParentIDs[comment.ID]; ok {
			filtered = append(filtered, comment)
		}
	}
	return filtered
}
