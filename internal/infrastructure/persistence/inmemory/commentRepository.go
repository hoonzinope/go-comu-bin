package inmemory

import (
	"context"
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

func (r *CommentRepository) Save(ctx context.Context, comment *entity.Comment) (int64, error) {
	_ = ctx
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

func (r *CommentRepository) SelectCommentByID(ctx context.Context, id int64) (*entity.Comment, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectCommentByID(id)
}

func (r *CommentRepository) SelectCommentByUUID(ctx context.Context, commentUUID string) (*entity.Comment, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectCommentByUUID(commentUUID)
}

func (r *CommentRepository) SelectCommentUUIDsByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]string, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectCommentUUIDsByIDsIncludingDeleted(ids)
}

func (r *CommentRepository) selectCommentByID(id int64) (*entity.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if comment, exists := r.commentDB.Data[id]; exists && comment.Status == entity.CommentStatusActive {
		return cloneComment(comment), nil
	}
	return nil, nil
}

func (r *CommentRepository) selectCommentByUUID(commentUUID string) (*entity.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, comment := range r.commentDB.Data {
		if comment.UUID == commentUUID {
			return cloneComment(comment), nil
		}
	}
	return nil, nil
}

func (r *CommentRepository) selectCommentUUIDsByIDsIncludingDeleted(ids []int64) (map[int64]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[int64]string, len(ids))
	for _, id := range ids {
		if comment, exists := r.commentDB.Data[id]; exists {
			out[id] = comment.UUID
		}
	}
	return out, nil
}

func (r *CommentRepository) SelectComments(ctx context.Context, postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	_ = ctx
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

func (r *CommentRepository) SelectCommentsIncludingDeleted(ctx context.Context, postID int64) ([]*entity.Comment, error) {
	_ = ctx
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

func (r *CommentRepository) SelectVisibleComments(ctx context.Context, postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()
	return r.selectVisibleComments(postID, limit, lastID)
}

func (r *CommentRepository) ExistsByAuthorIDIncludingDeleted(ctx context.Context, authorID int64) (bool, error) {
	_ = ctx
	r.coordinator.enter()
	defer r.coordinator.exit()

	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, comment := range r.commentDB.Data {
		if comment.AuthorID == authorID {
			return true, nil
		}
	}
	return false, nil
}

func (r *CommentRepository) selectVisibleComments(postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	comments, err := r.selectCommentsIncludingDeleted(postID)
	if err != nil {
		return nil, err
	}
	filtered := filterVisibleCommentsForRepository(comments, lastID)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered, nil
}

func (r *CommentRepository) Update(ctx context.Context, comment *entity.Comment) error {
	_ = ctx
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

func (r *CommentRepository) Delete(ctx context.Context, id int64) error {
	_ = ctx
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

func filterVisibleCommentsForRepository(comments []*entity.Comment, lastID int64) []*entity.Comment {
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
