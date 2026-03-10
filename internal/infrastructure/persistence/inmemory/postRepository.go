package inmemory

import (
	"errors"
	"sort"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.PostRepository = (*PostRepository)(nil)

type PostRepository struct {
	mu sync.RWMutex

	coordinator *txCoordinator

	postDB struct {
		ID   int64
		Data map[int64]*entity.Post
	}

	tagRepository     *TagRepository
	postTagRepository *PostTagRepository
}

type postRepositoryState struct {
	ID   int64
	Data map[int64]*entity.Post
}

func NewPostRepository(tagRepository *TagRepository, postTagRepository *PostTagRepository) *PostRepository {
	return &PostRepository{
		coordinator: newTxCoordinator(),
		postDB: struct {
			ID   int64
			Data map[int64]*entity.Post
		}{
			ID:   0,
			Data: make(map[int64]*entity.Post),
		},
		tagRepository:     tagRepository,
		postTagRepository: postTagRepository,
	}
}

func (r *PostRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *PostRepository) Save(post *entity.Post) (int64, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(post)
}

func (r *PostRepository) save(post *entity.Post) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.postDB.ID++
	saved := clonePost(post)
	saved.ID = r.postDB.ID
	r.postDB.Data[saved.ID] = saved
	post.ID = saved.ID
	return saved.ID, nil
}

func (r *PostRepository) SelectPostByID(id int64) (*entity.Post, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectPostByID(id)
}

func (r *PostRepository) selectPostByID(id int64) (*entity.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if post, exists := r.postDB.Data[id]; exists && post.Status == entity.PostStatusPublished {
		return clonePost(post), nil
	}
	return nil, nil
}

func (r *PostRepository) SelectPostByIDIncludingUnpublished(id int64) (*entity.Post, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectPostByIDIncludingUnpublished(id)
}

func (r *PostRepository) selectPostByIDIncludingUnpublished(id int64) (*entity.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if post, exists := r.postDB.Data[id]; exists && post.Status != entity.PostStatusDeleted {
		return clonePost(post), nil
	}
	return nil, nil
}

func (r *PostRepository) SelectPosts(boardID int64, limit int, lastID int64) ([]*entity.Post, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectPosts(boardID, limit, lastID)
}

func (r *PostRepository) selectPosts(boardID int64, limit int, lastID int64) ([]*entity.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		return []*entity.Post{}, nil
	}

	var posts []*entity.Post
	for _, post := range r.postDB.Data {
		if post.BoardID == boardID && post.Status == entity.PostStatusPublished {
			if lastID > 0 && post.ID >= lastID {
				continue
			}
			posts = append(posts, clonePost(post))
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

func (r *PostRepository) SelectPublishedPostsByTagName(tagName string, limit int, lastID int64) ([]*entity.Post, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectPublishedPostsByTagName(tagName, limit, lastID)
}

func (r *PostRepository) selectPublishedPostsByTagName(tagName string, limit int, lastID int64) ([]*entity.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if limit <= 0 {
		return []*entity.Post{}, nil
	}
	if r.tagRepository == nil || r.postTagRepository == nil {
		return nil, errors.New("post repository tag dependencies are not attached")
	}

	tag, err := r.tagRepository.selectByName(tagName)
	if err != nil {
		return nil, err
	}
	if tag == nil {
		return []*entity.Post{}, nil
	}
	activePostIDs := r.postTagRepository.activePostIDsByTagID(tag.ID)
	posts := make([]*entity.Post, 0, limit)
	for _, post := range r.postDB.Data {
		if post.Status != entity.PostStatusPublished {
			continue
		}
		if lastID > 0 && post.ID >= lastID {
			continue
		}
		if _, exists := activePostIDs[post.ID]; !exists {
			continue
		}
		posts = append(posts, clonePost(post))
	}
	sort.Slice(posts, func(i, j int) bool {
		return posts[i].ID > posts[j].ID
	})
	if len(posts) > limit {
		posts = posts[:limit]
	}
	return posts, nil
}

func (r *PostRepository) ExistsByBoardID(boardID int64) (bool, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.existsByBoardID(boardID)
}

func (r *PostRepository) existsByBoardID(boardID int64) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, post := range r.postDB.Data {
		if post.BoardID == boardID && post.Status != entity.PostStatusDeleted {
			return true, nil
		}
	}
	return false, nil
}

func (r *PostRepository) Update(post *entity.Post) error {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.update(post)
}

func (r *PostRepository) update(post *entity.Post) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.postDB.Data[post.ID]; exists {
		r.postDB.Data[post.ID] = clonePost(post)
		return nil
	}
	return nil
}

func (r *PostRepository) Delete(id int64) error {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.delete(id)
}

func (r *PostRepository) delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	post, exists := r.postDB.Data[id]
	if !exists {
		return nil
	}
	deleted := clonePost(post)
	deleted.SoftDelete()
	r.postDB.Data[id] = deleted
	return nil
}

func (r *PostRepository) snapshot() postRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := postRepositoryState{
		ID:   r.postDB.ID,
		Data: make(map[int64]*entity.Post, len(r.postDB.Data)),
	}
	for id, post := range r.postDB.Data {
		state.Data[id] = clonePost(post)
	}
	return state
}

func (r *PostRepository) restore(state postRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.postDB.ID = state.ID
	r.postDB.Data = make(map[int64]*entity.Post, len(state.Data))
	for id, post := range state.Data {
		r.postDB.Data[id] = clonePost(post)
	}
}

func clonePost(post *entity.Post) *entity.Post {
	if post == nil {
		return nil
	}
	out := *post
	if post.DeletedAt != nil {
		deletedAt := *post.DeletedAt
		out.DeletedAt = &deletedAt
	}
	return &out
}
