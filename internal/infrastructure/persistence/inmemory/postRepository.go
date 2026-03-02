package inmemory

import (
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type PostRepository struct {
	postDB struct {
		ID   int64
		Data map[int64]*entity.Post
	}
}

func NewPostRepository() *PostRepository {
	return &PostRepository{
		postDB: struct {
			ID   int64
			Data map[int64]*entity.Post
		}{
			ID:   0,
			Data: make(map[int64]*entity.Post),
		},
	}
}

func (r *PostRepository) Save(post *entity.Post) (int64, error) {
	r.postDB.ID++
	post.ID = r.postDB.ID
	r.postDB.Data[post.ID] = post
	return post.ID, nil
}

func (r *PostRepository) SelectPostByID(id int64) (*entity.Post, error) {
	if post, exists := r.postDB.Data[id]; exists {
		return post, nil
	}
	return nil, nil
}

func (r *PostRepository) SelectPosts(boardID int64, limit, offset int) ([]*entity.Post, error) {
	var posts []*entity.Post
	for _, post := range r.postDB.Data {
		if post.BoardID == boardID {
			posts = append(posts, post)
		}
	}
	if offset > len(posts) {
		return []*entity.Post{}, nil
	}
	end := offset + limit
	if end > len(posts) {
		end = len(posts)
	}
	return posts[offset:end], nil
}

func (r *PostRepository) Update(post *entity.Post) error {
	if _, exists := r.postDB.Data[post.ID]; exists {
		r.postDB.Data[post.ID] = post
		return nil
	}
	return nil
}

func (r *PostRepository) Delete(id int64) error {
	delete(r.postDB.Data, id)
	return nil
}
