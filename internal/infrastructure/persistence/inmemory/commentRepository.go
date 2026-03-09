package inmemory

import (
	"sort"
	"sync"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.CommentRepository = (*CommentRepository)(nil)

type CommentRepository struct {
	mu          sync.RWMutex
	coordinator *txCoordinator
	commentDB   struct {
		ID   int64
		Data map[int64]*entity.Comment
	}
}

type commentRepositoryState struct {
	ID   int64
	Data map[int64]*entity.Comment
}

func NewCommentRepository() *CommentRepository {
	return &CommentRepository{
		coordinator: newTxCoordinator(),
		commentDB: struct {
			ID   int64
			Data map[int64]*entity.Comment
		}{
			ID:   0,
			Data: make(map[int64]*entity.Comment),
		},
	}
}

func (r *CommentRepository) attachCoordinator(coordinator *txCoordinator) {
	r.coordinator = coordinator
}

func (r *CommentRepository) Save(comment *entity.Comment) (int64, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.save(comment)
}

func (r *CommentRepository) save(comment *entity.Comment) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.commentDB.ID++
	saved := cloneComment(comment)
	saved.ID = r.commentDB.ID
	r.commentDB.Data[saved.ID] = saved
	comment.ID = saved.ID
	return saved.ID, nil
}

func (r *CommentRepository) SelectCommentByID(id int64) (*entity.Comment, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectCommentByID(id)
}

func (r *CommentRepository) selectCommentByID(id int64) (*entity.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if comment, exists := r.commentDB.Data[id]; exists && comment.Status == entity.CommentStatusActive {
		return cloneComment(comment), nil
	}
	return nil, nil
}

func (r *CommentRepository) SelectComments(postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectComments(postID, limit, lastID)
}

func (r *CommentRepository) selectComments(postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
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
			comments = append(comments, cloneComment(comment))
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
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectCommentsIncludingDeleted(postID)
}

func (r *CommentRepository) selectCommentsIncludingDeleted(postID int64) ([]*entity.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var comments []*entity.Comment
	for _, comment := range r.commentDB.Data {
		if comment.PostID == postID {
			comments = append(comments, cloneComment(comment))
		}
	}
	sort.Slice(comments, func(i, j int) bool {
		return comments[i].ID > comments[j].ID
	})
	return comments, nil
}

func (r *CommentRepository) Update(comment *entity.Comment) error {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.update(comment)
}

func (r *CommentRepository) update(comment *entity.Comment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.commentDB.Data[comment.ID]; exists {
		r.commentDB.Data[comment.ID] = cloneComment(comment)
		return nil
	}
	return nil
}

func (r *CommentRepository) Delete(id int64) error {
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.delete(id)
}

func (r *CommentRepository) delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	comment, exists := r.commentDB.Data[id]
	if !exists {
		return nil
	}
	deleted := cloneComment(comment)
	deleted.SoftDelete()
	r.commentDB.Data[id] = deleted
	return nil
}

func (r *CommentRepository) snapshot() commentRepositoryState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	state := commentRepositoryState{
		ID:   r.commentDB.ID,
		Data: make(map[int64]*entity.Comment, len(r.commentDB.Data)),
	}
	for id, comment := range r.commentDB.Data {
		state.Data[id] = cloneComment(comment)
	}
	return state
}

func (r *CommentRepository) restore(state commentRepositoryState) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.commentDB.ID = state.ID
	r.commentDB.Data = make(map[int64]*entity.Comment, len(state.Data))
	for id, comment := range state.Data {
		r.commentDB.Data[id] = cloneComment(comment)
	}
}

func cloneComment(comment *entity.Comment) *entity.Comment {
	if comment == nil {
		return nil
	}
	out := *comment
	if comment.ParentID != nil {
		parentID := *comment.ParentID
		out.ParentID = &parentID
	}
	if comment.DeletedAt != nil {
		deletedAt := *comment.DeletedAt
		out.DeletedAt = &deletedAt
	}
	return &out
}
