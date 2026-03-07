package inmemory

import (
	"sort"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.PostRepository = (*PostRepository)(nil)

type PostRepository struct {
	mu     sync.RWMutex
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
	r.mu.Lock()
	defer r.mu.Unlock()

	r.postDB.ID++
	post.ID = r.postDB.ID
	r.postDB.Data[post.ID] = post
	return post.ID, nil
}

func (r *PostRepository) SelectPostByID(id int64) (*entity.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if post, exists := r.postDB.Data[id]; exists {
		return post, nil
	}
	return nil, nil
}

func (r *PostRepository) SelectPosts(boardID int64, limit int, lastID int64) ([]*entity.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		return []*entity.Post{}, nil
	}

	var posts []*entity.Post
	for _, post := range r.postDB.Data {
		if post.BoardID == boardID {
			if lastID > 0 && post.ID >= lastID {
				continue
			}
			posts = append(posts, post)
		}
	}
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].ID > posts[j].ID
	})

	if len(posts) > limit {
		posts = posts[:limit]
	}
	return posts, nil
}

func (r *PostRepository) ExistsByAuthor(authorID int64) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, post := range r.postDB.Data {
		if post.AuthorID == authorID {
			return true, nil
		}
	}
	return false, nil
}

func (r *PostRepository) Update(post *entity.Post) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.postDB.Data[post.ID]; exists {
		r.postDB.Data[post.ID] = post
		return nil
	}
	return nil
}

func (r *PostRepository) Delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.postDB.Data, id)
	return nil
}
