package inmemory

import (
	"sort"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.CommentRepository = (*CommentRepository)(nil)

type CommentRepository struct {
	mu        sync.RWMutex
	commentDB struct {
		ID   int64
		Data map[int64]*entity.Comment
	}
}

func NewCommentRepository() *CommentRepository {
	return &CommentRepository{
		commentDB: struct {
			ID   int64
			Data map[int64]*entity.Comment
		}{
			ID:   0,
			Data: make(map[int64]*entity.Comment),
		},
	}
}

func (r *CommentRepository) Save(comment *entity.Comment) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.commentDB.ID++
	comment.ID = r.commentDB.ID
	r.commentDB.Data[comment.ID] = comment
	return comment.ID, nil
}

func (r *CommentRepository) SelectCommentByID(id int64) (*entity.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if comment, exists := r.commentDB.Data[id]; exists && comment.Status == entity.CommentStatusActive {
		return comment, nil
	}
	return nil, nil
}

func (r *CommentRepository) SelectComments(postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		return []*entity.Comment{}, nil
	}

	var comments []*entity.Comment
	for _, comment := range r.commentDB.Data {
		if comment.PostID == postID && comment.Status == entity.CommentStatusActive {
			if lastID > 0 && comment.ID >= lastID {
				continue
			}
			comments = append(comments, comment)
		}
	}
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].ID > comments[j].ID
	})

	if len(comments) > limit {
		comments = comments[:limit]
	}
	return comments, nil
}

func (r *CommentRepository) SelectCommentsIncludingDeleted(postID int64) ([]*entity.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var comments []*entity.Comment
	for _, comment := range r.commentDB.Data {
		if comment.PostID == postID {
			comments = append(comments, comment)
		}
	}
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].ID > comments[j].ID
	})
	return comments, nil
}

func (r *CommentRepository) Update(comment *entity.Comment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.commentDB.Data[comment.ID]; exists {
		r.commentDB.Data[comment.ID] = comment
		return nil
	}
	return nil
}

func (r *CommentRepository) Delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	comment, exists := r.commentDB.Data[id]
	if !exists {
		return nil
	}
	comment.SoftDelete()
	r.commentDB.Data[id] = comment
	return nil
}
