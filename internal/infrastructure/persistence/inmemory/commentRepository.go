package inmemory

import (
	"sort"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ application.CommentRepository = (*CommentRepository)(nil)

type CommentRepository struct {
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
	r.commentDB.ID++
	comment.ID = r.commentDB.ID
	r.commentDB.Data[comment.ID] = comment
	return comment.ID, nil
}

func (r *CommentRepository) SelectCommentByID(id int64) (*entity.Comment, error) {
	if comment, exists := r.commentDB.Data[id]; exists {
		return comment, nil
	}
	return nil, nil
}

func (r *CommentRepository) SelectComments(postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	if limit <= 0 {
		return []*entity.Comment{}, nil
	}

	var comments []*entity.Comment
	for _, comment := range r.commentDB.Data {
		if comment.PostID == postID {
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

func (r *CommentRepository) Update(comment *entity.Comment) error {
	if _, exists := r.commentDB.Data[comment.ID]; exists {
		r.commentDB.Data[comment.ID] = comment
		return nil
	}
	return nil
}

func (r *CommentRepository) Delete(id int64) error {
	delete(r.commentDB.Data, id)
	return nil
}
