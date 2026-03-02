package inmemory

import (
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

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

func (r *CommentRepository) SelectComments(postID int64, limit, offset int) ([]*entity.Comment, error) {
	var comments []*entity.Comment
	for _, comment := range r.commentDB.Data {
		if comment.PostID == postID {
			comments = append(comments, comment)
		}
	}
	if offset > len(comments) {
		return []*entity.Comment{}, nil
	}
	end := offset + limit
	if end > len(comments) {
		end = len(comments)
	}
	return comments[offset:end], nil
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
