package inmemory

import (
	"context"
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.UnitOfWork = (*UnitOfWork)(nil)
var _ port.TxScope = (*txScope)(nil)

type UnitOfWork struct {
	mu                   sync.Mutex
	userRepository       *UserRepository
	boardRepository      *BoardRepository
	postRepository       *PostRepository
	tagRepository        *TagRepository
	postTagRepo          *PostTagRepository
	commentRepository    *CommentRepository
	reactionRepository   *ReactionRepository
	attachmentRepository *AttachmentRepository
	outboxRepository     *OutboxRepository
}

type txScope struct {
	ctx                  context.Context
	userRepository       port.UserRepository
	boardRepository      port.BoardRepository
	postRepository       port.PostRepository
	tagRepository        port.TagRepository
	postTagRepository    port.PostTagRepository
	commentRepository    port.CommentRepository
	reactionRepository   port.ReactionRepository
	attachmentRepository port.AttachmentRepository
	outboxAppender       port.OutboxAppender
}

func (s *txScope) Context() context.Context { return s.ctx }

func NewUnitOfWork(userRepository *UserRepository, boardRepository *BoardRepository, postRepository *PostRepository, tagRepository *TagRepository, postTagRepo *PostTagRepository, commentRepository *CommentRepository, reactionRepository *ReactionRepository, attachmentRepository *AttachmentRepository, outboxRepository *OutboxRepository) *UnitOfWork {
	userRepository.attachCoordinator(newTxCoordinator())
	boardRepository.attachCoordinator(newTxCoordinator())
	postRepository.attachCoordinator(newTxCoordinator())
	tagRepository.attachCoordinator(newTxCoordinator())
	postTagRepo.attachCoordinator(newTxCoordinator())
	commentRepository.attachCoordinator(newTxCoordinator())
	reactionRepository.attachCoordinator(newTxCoordinator())
	attachmentRepository.attachCoordinator(newTxCoordinator())
	outboxRepository.attachCoordinator(newTxCoordinator())

	return &UnitOfWork{
		userRepository:       userRepository,
		boardRepository:      boardRepository,
		postRepository:       postRepository,
		tagRepository:        tagRepository,
		postTagRepo:          postTagRepo,
		commentRepository:    commentRepository,
		reactionRepository:   reactionRepository,
		attachmentRepository: attachmentRepository,
		outboxRepository:     outboxRepository,
	}
}

func (u *UnitOfWork) WithinTransaction(ctx context.Context, fn func(tx port.TxScope) error) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	var (
		postState             postRepositoryState
		postSnapshotted       bool
		userState             userRepositoryState
		userSnapshotted       bool
		boardState            boardRepositoryState
		boardSnapshotted      bool
		tagState              tagRepositoryState
		tagSnapshotted        bool
		postTagState          postTagRepositoryState
		postTagSnapshotted    bool
		commentState          commentRepositoryState
		commentSnapshotted    bool
		reactionState         reactionRepositoryState
		reactionSnapshotted   bool
		attachmentState       attachmentRepositoryState
		attachmentSnapshotted bool
		outboxState           outboxRepositoryState
		outboxSnapshotted     bool
		userLocked            bool
		boardLocked           bool
		postLocked            bool
		tagLocked             bool
		postTagLocked         bool
		commentLocked         bool
		reactionLocked        bool
		attachmentLocked      bool
		outboxLocked          bool
	)
	defer func() {
		if outboxLocked {
			u.outboxRepository.coordinator.unlock()
		}
		if attachmentLocked {
			u.attachmentRepository.coordinator.unlock()
		}
		if reactionLocked {
			u.reactionRepository.coordinator.unlock()
		}
		if commentLocked {
			u.commentRepository.coordinator.unlock()
		}
		if postTagLocked {
			u.postTagRepo.coordinator.unlock()
		}
		if tagLocked {
			u.tagRepository.coordinator.unlock()
		}
		if postLocked {
			u.postRepository.coordinator.unlock()
		}
		if boardLocked {
			u.boardRepository.coordinator.unlock()
		}
		if userLocked {
			u.userRepository.coordinator.unlock()
		}
	}()

	capturePost := func() {
		if !postLocked {
			u.postRepository.coordinator.lock()
			postLocked = true
		}
		if postSnapshotted {
			return
		}
		postState = u.postRepository.snapshot()
		postSnapshotted = true
	}
	captureUser := func() {
		if !userLocked {
			u.userRepository.coordinator.lock()
			userLocked = true
		}
		if userSnapshotted {
			return
		}
		userState = u.userRepository.snapshot()
		userSnapshotted = true
	}
	captureBoard := func() {
		if !boardLocked {
			u.boardRepository.coordinator.lock()
			boardLocked = true
		}
		if boardSnapshotted {
			return
		}
		boardState = u.boardRepository.snapshot()
		boardSnapshotted = true
	}
	captureTag := func() {
		if !tagLocked {
			u.tagRepository.coordinator.lock()
			tagLocked = true
		}
		if tagSnapshotted {
			return
		}
		tagState = u.tagRepository.snapshot()
		tagSnapshotted = true
	}
	capturePostTag := func() {
		if !postTagLocked {
			u.postTagRepo.coordinator.lock()
			postTagLocked = true
		}
		if postTagSnapshotted {
			return
		}
		postTagState = u.postTagRepo.snapshot()
		postTagSnapshotted = true
	}
	captureComment := func() {
		if !commentLocked {
			u.commentRepository.coordinator.lock()
			commentLocked = true
		}
		if commentSnapshotted {
			return
		}
		commentState = u.commentRepository.snapshot()
		commentSnapshotted = true
	}
	captureReaction := func() {
		if !reactionLocked {
			u.reactionRepository.coordinator.lock()
			reactionLocked = true
		}
		if reactionSnapshotted {
			return
		}
		reactionState = u.reactionRepository.snapshot()
		reactionSnapshotted = true
	}
	captureAttachment := func() {
		if !attachmentLocked {
			u.attachmentRepository.coordinator.lock()
			attachmentLocked = true
		}
		if attachmentSnapshotted {
			return
		}
		attachmentState = u.attachmentRepository.snapshot()
		attachmentSnapshotted = true
	}
	captureOutbox := func() {
		if !outboxLocked {
			u.outboxRepository.coordinator.lock()
			outboxLocked = true
		}
		if outboxSnapshotted {
			return
		}
		outboxState = u.outboxRepository.snapshot()
		outboxSnapshotted = true
	}

	tx := &txScope{
		ctx:                  ctx,
		userRepository:       userTxRepository{repo: u.userRepository, beforeWrite: captureUser},
		boardRepository:      boardTxRepository{repo: u.boardRepository, beforeWrite: captureBoard},
		postRepository:       postTxRepository{repo: u.postRepository, beforeWrite: capturePost},
		tagRepository:        tagTxRepository{repo: u.tagRepository, beforeWrite: captureTag},
		postTagRepository:    postTagTxRepository{repo: u.postTagRepo, beforeWrite: capturePostTag},
		commentRepository:    commentTxRepository{repo: u.commentRepository, beforeWrite: captureComment},
		reactionRepository:   reactionTxRepository{repo: u.reactionRepository, beforeWrite: captureReaction},
		attachmentRepository: attachmentTxRepository{repo: u.attachmentRepository, beforeWrite: captureAttachment},
		outboxAppender:       outboxTxAppender{repo: u.outboxRepository, beforeWrite: captureOutbox},
	}
	if err := fn(tx); err != nil {
		if userSnapshotted {
			u.userRepository.restore(userState)
		}
		if boardSnapshotted {
			u.boardRepository.restore(boardState)
		}
		if postSnapshotted {
			u.postRepository.restore(postState)
		}
		if tagSnapshotted {
			u.tagRepository.restore(tagState)
		}
		if postTagSnapshotted {
			u.postTagRepo.restore(postTagState)
		}
		if commentSnapshotted {
			u.commentRepository.restore(commentState)
		}
		if reactionSnapshotted {
			u.reactionRepository.restore(reactionState)
		}
		if attachmentSnapshotted {
			u.attachmentRepository.restore(attachmentState)
		}
		if outboxSnapshotted {
			u.outboxRepository.restore(outboxState)
		}
		return err
	}
	return nil
}

func (t *txScope) UserRepository() port.UserRepository {
	return t.userRepository
}

func (t *txScope) BoardRepository() port.BoardRepository {
	return t.boardRepository
}

func (t *txScope) PostRepository() port.PostRepository {
	return t.postRepository
}

func (t *txScope) TagRepository() port.TagRepository {
	return t.tagRepository
}

func (t *txScope) PostTagRepository() port.PostTagRepository {
	return t.postTagRepository
}

func (t *txScope) CommentRepository() port.CommentRepository {
	return t.commentRepository
}

func (t *txScope) ReactionRepository() port.ReactionRepository {
	return t.reactionRepository
}

func (t *txScope) AttachmentRepository() port.AttachmentRepository {
	return t.attachmentRepository
}

func (t *txScope) Outbox() port.OutboxAppender {
	return t.outboxAppender
}

type postTxRepository struct {
	repo        *PostRepository
	beforeWrite func()
}

type userTxRepository struct {
	repo        *UserRepository
	beforeWrite func()
}

func (r userTxRepository) Save(user *entity.User) (int64, error) {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(user)
}
func (r userTxRepository) SelectUserByUsername(username string) (*entity.User, error) {
	return r.repo.selectUserByUsername(username)
}
func (r userTxRepository) SelectUserByUUID(userUUID string) (*entity.User, error) {
	return r.repo.selectUserByUUID(userUUID)
}
func (r userTxRepository) SelectUserByID(id int64) (*entity.User, error) {
	return r.repo.selectUserByID(id)
}
func (r userTxRepository) SelectUserByIDIncludingDeleted(id int64) (*entity.User, error) {
	return r.repo.selectUserByIDIncludingDeleted(id)
}
func (r userTxRepository) SelectUsersByIDsIncludingDeleted(ids []int64) (map[int64]*entity.User, error) {
	return r.repo.selectUsersByIDsIncludingDeleted(ids)
}
func (r userTxRepository) Update(user *entity.User) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(user)
}
func (r userTxRepository) Delete(id int64) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.delete(id)
}

type boardTxRepository struct {
	repo        *BoardRepository
	beforeWrite func()
}

func (r boardTxRepository) SelectBoardByID(id int64) (*entity.Board, error) {
	return r.repo.selectBoardByID(id)
}
func (r boardTxRepository) SelectBoardList(limit int, lastID int64) ([]*entity.Board, error) {
	return r.repo.selectBoardList(limit, lastID)
}
func (r boardTxRepository) Save(board *entity.Board) (int64, error) {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(board)
}
func (r boardTxRepository) Update(board *entity.Board) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(board)
}
func (r boardTxRepository) Delete(id int64) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.delete(id)
}

func (r postTxRepository) Save(post *entity.Post) (int64, error) {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(post)
}
func (r postTxRepository) SelectPostByID(id int64) (*entity.Post, error) {
	return r.repo.selectPostByID(id)
}
func (r postTxRepository) SelectPostByIDIncludingUnpublished(id int64) (*entity.Post, error) {
	return r.repo.selectPostByIDIncludingUnpublished(id)
}
func (r postTxRepository) SelectPosts(boardID int64, limit int, lastID int64) ([]*entity.Post, error) {
	return r.repo.selectPosts(boardID, limit, lastID)
}
func (r postTxRepository) SelectPublishedPostsByTagName(tagName string, limit int, lastID int64) ([]*entity.Post, error) {
	return r.repo.selectPublishedPostsByTagName(tagName, limit, lastID)
}
func (r postTxRepository) ExistsByBoardID(boardID int64) (bool, error) {
	return r.repo.existsByBoardID(boardID)
}
func (r postTxRepository) Update(post *entity.Post) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(post)
}
func (r postTxRepository) Delete(id int64) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.delete(id)
}

type tagTxRepository struct {
	repo        *TagRepository
	beforeWrite func()
}

func (r tagTxRepository) Save(tag *entity.Tag) (int64, error) {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(tag)
}
func (r tagTxRepository) SelectByName(name string) (*entity.Tag, error) {
	return r.repo.selectByName(name)
}
func (r tagTxRepository) SelectByIDs(ids []int64) ([]*entity.Tag, error) {
	return r.repo.selectByIDs(ids)
}

type postTagTxRepository struct {
	repo        *PostTagRepository
	beforeWrite func()
}

func (r postTagTxRepository) SelectActiveByPostID(postID int64) ([]*entity.PostTag, error) {
	return r.repo.selectActiveByPostID(postID)
}
func (r postTagTxRepository) SelectActiveByTagID(tagID int64, limit int, lastID int64) ([]*entity.PostTag, error) {
	return r.repo.selectActiveByTagID(tagID, limit, lastID)
}
func (r postTagTxRepository) UpsertActive(postID, tagID int64) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.upsertActive(postID, tagID)
}
func (r postTagTxRepository) SoftDelete(postID, tagID int64) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.softDelete(postID, tagID)
}
func (r postTagTxRepository) SoftDeleteByPostID(postID int64) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.softDeleteByPostID(postID)
}

type commentTxRepository struct {
	repo        *CommentRepository
	beforeWrite func()
}

func (r commentTxRepository) Save(comment *entity.Comment) (int64, error) {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(comment)
}
func (r commentTxRepository) SelectCommentByID(id int64) (*entity.Comment, error) {
	return r.repo.selectCommentByID(id)
}
func (r commentTxRepository) SelectComments(postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	return r.repo.selectComments(postID, limit, lastID)
}
func (r commentTxRepository) SelectCommentsIncludingDeleted(postID int64) ([]*entity.Comment, error) {
	return r.repo.selectCommentsIncludingDeleted(postID)
}
func (r commentTxRepository) SelectVisibleComments(postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	return r.repo.selectVisibleComments(postID, limit, lastID)
}
func (r commentTxRepository) Update(comment *entity.Comment) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(comment)
}
func (r commentTxRepository) Delete(id int64) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.delete(id)
}

type reactionTxRepository struct {
	repo        *ReactionRepository
	beforeWrite func()
}

func (r reactionTxRepository) SetUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (*entity.Reaction, bool, bool, error) {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.setUserTargetReaction(userID, targetID, targetType, reactionType)
}
func (r reactionTxRepository) DeleteUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType) (bool, error) {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.deleteUserTargetReaction(userID, targetID, targetType)
}
func (r reactionTxRepository) DeleteByTarget(targetID int64, targetType entity.ReactionTargetType) (int, error) {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.deleteByTarget(targetID, targetType)
}
func (r reactionTxRepository) GetUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType) (*entity.Reaction, error) {
	return r.repo.getUserTargetReaction(userID, targetID, targetType)
}
func (r reactionTxRepository) GetByTarget(targetID int64, targetType entity.ReactionTargetType) ([]*entity.Reaction, error) {
	return r.repo.getByTarget(targetID, targetType)
}
func (r reactionTxRepository) GetByTargets(targetIDs []int64, targetType entity.ReactionTargetType) (map[int64][]*entity.Reaction, error) {
	return r.repo.getByTargets(targetIDs, targetType)
}

type attachmentTxRepository struct {
	repo        *AttachmentRepository
	beforeWrite func()
}

type outboxTxAppender struct {
	repo        *OutboxRepository
	beforeWrite func()
}

func (r attachmentTxRepository) Save(attachment *entity.Attachment) (int64, error) {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(attachment)
}
func (r attachmentTxRepository) SelectByID(id int64) (*entity.Attachment, error) {
	return r.repo.selectByID(id)
}
func (r attachmentTxRepository) SelectByPostID(postID int64) ([]*entity.Attachment, error) {
	return r.repo.selectByPostID(postID)
}
func (r attachmentTxRepository) SelectCleanupCandidatesBefore(cutoff time.Time, limit int) ([]*entity.Attachment, error) {
	return r.repo.selectCleanupCandidatesBefore(cutoff, limit)
}
func (r attachmentTxRepository) Update(attachment *entity.Attachment) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(attachment)
}
func (r attachmentTxRepository) Delete(id int64) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.delete(id)
}

func (r outboxTxAppender) Append(messages ...port.OutboxMessage) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.append(messages...)
}
