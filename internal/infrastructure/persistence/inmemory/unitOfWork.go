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
	reportRepository     *ReportRepository
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
	reportRepository     port.ReportRepository
	outboxAppender       port.OutboxAppender
}

func (s *txScope) Context() context.Context { return s.ctx }

func NewUnitOfWork(userRepository *UserRepository, boardRepository *BoardRepository, postRepository *PostRepository, tagRepository *TagRepository, postTagRepo *PostTagRepository, commentRepository *CommentRepository, reactionRepository *ReactionRepository, attachmentRepository *AttachmentRepository, reportRepository *ReportRepository, outboxRepository *OutboxRepository) *UnitOfWork {
	userRepository.attachCoordinator(newTxCoordinator())
	boardRepository.attachCoordinator(newTxCoordinator())
	postRepository.attachCoordinator(newTxCoordinator())
	tagRepository.attachCoordinator(newTxCoordinator())
	postTagRepo.attachCoordinator(newTxCoordinator())
	commentRepository.attachCoordinator(newTxCoordinator())
	reactionRepository.attachCoordinator(newTxCoordinator())
	attachmentRepository.attachCoordinator(newTxCoordinator())
	reportRepository.attachCoordinator(newTxCoordinator())
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
		reportRepository:     reportRepository,
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
		reportState           reportRepositoryState
		reportSnapshotted     bool
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
		reportLocked          bool
		outboxLocked          bool
	)
	defer func() {
		if outboxLocked {
			u.outboxRepository.coordinator.unlock()
		}
		if reportLocked {
			u.reportRepository.coordinator.unlock()
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
	captureReport := func() {
		if !reportLocked {
			u.reportRepository.coordinator.lock()
			reportLocked = true
		}
		if reportSnapshotted {
			return
		}
		reportState = u.reportRepository.snapshot()
		reportSnapshotted = true
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
		reportRepository:     reportTxRepository{repo: u.reportRepository, beforeWrite: captureReport},
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
		if reportSnapshotted {
			u.reportRepository.restore(reportState)
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

func (t *txScope) ReportRepository() port.ReportRepository {
	return t.reportRepository
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

func (r userTxRepository) Save(ctx context.Context, user *entity.User) (int64, error) {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(user)
}
func (r userTxRepository) SelectUserByUsername(ctx context.Context, username string) (*entity.User, error) {
	_ = ctx
	return r.repo.selectUserByUsername(username)
}
func (r userTxRepository) SelectUserByUUID(ctx context.Context, userUUID string) (*entity.User, error) {
	_ = ctx
	return r.repo.selectUserByUUID(userUUID)
}
func (r userTxRepository) SelectUserByID(ctx context.Context, id int64) (*entity.User, error) {
	_ = ctx
	return r.repo.selectUserByID(id)
}
func (r userTxRepository) SelectUserByIDIncludingDeleted(ctx context.Context, id int64) (*entity.User, error) {
	_ = ctx
	return r.repo.selectUserByIDIncludingDeleted(id)
}
func (r userTxRepository) SelectUsersByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]*entity.User, error) {
	_ = ctx
	return r.repo.selectUsersByIDsIncludingDeleted(ids)
}
func (r userTxRepository) SelectGuestCleanupCandidates(ctx context.Context, now time.Time, pendingGrace, activeUnusedGrace time.Duration, limit int) ([]*entity.User, error) {
	_ = ctx
	return r.repo.selectGuestCleanupCandidates(now, pendingGrace, activeUnusedGrace, limit)
}
func (r userTxRepository) Update(ctx context.Context, user *entity.User) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(user)
}
func (r userTxRepository) Delete(ctx context.Context, id int64) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.delete(id)
}

type boardTxRepository struct {
	repo        *BoardRepository
	beforeWrite func()
}

func (r boardTxRepository) SelectBoardByID(ctx context.Context, id int64) (*entity.Board, error) {
	_ = ctx
	return r.repo.selectBoardByID(id)
}
func (r boardTxRepository) SelectBoardByUUID(ctx context.Context, boardUUID string) (*entity.Board, error) {
	_ = ctx
	return r.repo.selectBoardByUUID(boardUUID)
}
func (r boardTxRepository) SelectBoardsByIDs(ctx context.Context, ids []int64) (map[int64]*entity.Board, error) {
	_ = ctx
	return r.repo.selectBoardsByIDs(ids)
}
func (r boardTxRepository) SelectBoardList(ctx context.Context, limit int, lastID int64) ([]*entity.Board, error) {
	_ = ctx
	return r.repo.selectBoardList(limit, lastID)
}
func (r boardTxRepository) Save(ctx context.Context, board *entity.Board) (int64, error) {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(board)
}
func (r boardTxRepository) Update(ctx context.Context, board *entity.Board) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(board)
}
func (r boardTxRepository) Delete(ctx context.Context, id int64) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.delete(id)
}

func (r postTxRepository) Save(ctx context.Context, post *entity.Post) (int64, error) {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(post)
}
func (r postTxRepository) SelectPostByID(ctx context.Context, id int64) (*entity.Post, error) {
	_ = ctx
	return r.repo.selectPostByID(id)
}
func (r postTxRepository) SelectPostByUUID(ctx context.Context, postUUID string) (*entity.Post, error) {
	_ = ctx
	return r.repo.selectPostByUUID(postUUID)
}
func (r postTxRepository) SelectPostUUIDsByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	_ = ctx
	return r.repo.SelectPostUUIDsByIDs(ctx, ids)
}
func (r postTxRepository) SelectPostUUIDsByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]string, error) {
	_ = ctx
	return r.repo.SelectPostUUIDsByIDsIncludingDeleted(ctx, ids)
}
func (r postTxRepository) SelectPostByIDIncludingUnpublished(ctx context.Context, id int64) (*entity.Post, error) {
	_ = ctx
	return r.repo.selectPostByIDIncludingUnpublished(id)
}
func (r postTxRepository) SelectPostByUUIDIncludingUnpublished(ctx context.Context, postUUID string) (*entity.Post, error) {
	_ = ctx
	return r.repo.selectPostByUUIDIncludingUnpublished(postUUID)
}
func (r postTxRepository) SelectPosts(ctx context.Context, boardID int64, limit int, lastID int64) ([]*entity.Post, error) {
	_ = ctx
	return r.repo.selectPosts(boardID, limit, lastID)
}
func (r postTxRepository) SelectPublishedPostsByTagName(ctx context.Context, tagName string, limit int, lastID int64) ([]*entity.Post, error) {
	return r.repo.selectPublishedPostsByTagName(ctx, tagName, limit, lastID)
}
func (r postTxRepository) ExistsByBoardID(ctx context.Context, boardID int64) (bool, error) {
	_ = ctx
	return r.repo.existsByBoardID(boardID)
}
func (r postTxRepository) ExistsByAuthorID(ctx context.Context, authorID int64) (bool, error) {
	_ = ctx
	return r.repo.ExistsByAuthorID(ctx, authorID)
}
func (r postTxRepository) ExistsByAuthorIDIncludingDeleted(ctx context.Context, authorID int64) (bool, error) {
	_ = ctx
	return r.repo.ExistsByAuthorIDIncludingDeleted(ctx, authorID)
}
func (r postTxRepository) Update(ctx context.Context, post *entity.Post) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(post)
}
func (r postTxRepository) Delete(ctx context.Context, id int64) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.delete(id)
}

type tagTxRepository struct {
	repo        *TagRepository
	beforeWrite func()
}

func (r tagTxRepository) Save(ctx context.Context, tag *entity.Tag) (int64, error) {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(tag)
}
func (r tagTxRepository) SelectByName(ctx context.Context, name string) (*entity.Tag, error) {
	_ = ctx
	return r.repo.selectByName(name)
}
func (r tagTxRepository) SelectByIDs(ctx context.Context, ids []int64) ([]*entity.Tag, error) {
	_ = ctx
	return r.repo.selectByIDs(ids)
}

type postTagTxRepository struct {
	repo        *PostTagRepository
	beforeWrite func()
}

func (r postTagTxRepository) SelectActiveByPostID(ctx context.Context, postID int64) ([]*entity.PostTag, error) {
	_ = ctx
	return r.repo.selectActiveByPostID(postID)
}
func (r postTagTxRepository) SelectActiveByTagID(ctx context.Context, tagID int64, limit int, lastID int64) ([]*entity.PostTag, error) {
	_ = ctx
	return r.repo.selectActiveByTagID(tagID, limit, lastID)
}
func (r postTagTxRepository) UpsertActive(ctx context.Context, postID, tagID int64) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.upsertActive(postID, tagID)
}
func (r postTagTxRepository) SoftDelete(ctx context.Context, postID, tagID int64) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.softDelete(postID, tagID)
}
func (r postTagTxRepository) SoftDeleteByPostID(ctx context.Context, postID int64) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.softDeleteByPostID(postID)
}

type commentTxRepository struct {
	repo        *CommentRepository
	beforeWrite func()
}

func (r commentTxRepository) Save(ctx context.Context, comment *entity.Comment) (int64, error) {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(comment)
}
func (r commentTxRepository) SelectCommentByID(ctx context.Context, id int64) (*entity.Comment, error) {
	_ = ctx
	return r.repo.selectCommentByID(id)
}
func (r commentTxRepository) SelectCommentByUUID(ctx context.Context, commentUUID string) (*entity.Comment, error) {
	_ = ctx
	return r.repo.selectCommentByUUID(commentUUID)
}
func (r commentTxRepository) SelectCommentUUIDsByIDsIncludingDeleted(ctx context.Context, ids []int64) (map[int64]string, error) {
	_ = ctx
	return r.repo.selectCommentUUIDsByIDsIncludingDeleted(ids)
}
func (r commentTxRepository) SelectComments(ctx context.Context, postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	_ = ctx
	return r.repo.selectComments(postID, limit, lastID)
}
func (r commentTxRepository) SelectCommentsIncludingDeleted(ctx context.Context, postID int64) ([]*entity.Comment, error) {
	_ = ctx
	return r.repo.selectCommentsIncludingDeleted(postID)
}
func (r commentTxRepository) SelectVisibleComments(ctx context.Context, postID int64, limit int, lastID int64) ([]*entity.Comment, error) {
	_ = ctx
	return r.repo.selectVisibleComments(postID, limit, lastID)
}
func (r commentTxRepository) ExistsByAuthorID(ctx context.Context, authorID int64) (bool, error) {
	_ = ctx
	return r.repo.ExistsByAuthorID(ctx, authorID)
}
func (r commentTxRepository) ExistsByAuthorIDIncludingDeleted(ctx context.Context, authorID int64) (bool, error) {
	_ = ctx
	return r.repo.ExistsByAuthorIDIncludingDeleted(ctx, authorID)
}
func (r commentTxRepository) Update(ctx context.Context, comment *entity.Comment) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(comment)
}
func (r commentTxRepository) Delete(ctx context.Context, id int64) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.delete(id)
}

type reactionTxRepository struct {
	repo        *ReactionRepository
	beforeWrite func()
}

func (r reactionTxRepository) SetUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (*entity.Reaction, bool, bool, error) {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.setUserTargetReaction(userID, targetID, targetType, reactionType)
}
func (r reactionTxRepository) DeleteUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType) (bool, error) {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.deleteUserTargetReaction(userID, targetID, targetType)
}
func (r reactionTxRepository) DeleteByTarget(ctx context.Context, targetID int64, targetType entity.ReactionTargetType) (int, error) {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.deleteByTarget(targetID, targetType)
}
func (r reactionTxRepository) GetUserTargetReaction(ctx context.Context, userID, targetID int64, targetType entity.ReactionTargetType) (*entity.Reaction, error) {
	_ = ctx
	return r.repo.getUserTargetReaction(userID, targetID, targetType)
}
func (r reactionTxRepository) GetByTarget(ctx context.Context, targetID int64, targetType entity.ReactionTargetType) ([]*entity.Reaction, error) {
	_ = ctx
	return r.repo.getByTarget(targetID, targetType)
}
func (r reactionTxRepository) GetByTargets(ctx context.Context, targetIDs []int64, targetType entity.ReactionTargetType) (map[int64][]*entity.Reaction, error) {
	_ = ctx
	return r.repo.getByTargets(targetIDs, targetType)
}
func (r reactionTxRepository) ExistsByUserID(ctx context.Context, userID int64) (bool, error) {
	_ = ctx
	return r.repo.ExistsByUserID(ctx, userID)
}

type attachmentTxRepository struct {
	repo        *AttachmentRepository
	beforeWrite func()
}

type reportTxRepository struct {
	repo        *ReportRepository
	beforeWrite func()
}

type outboxTxAppender struct {
	repo        *OutboxRepository
	beforeWrite func()
}

func (r attachmentTxRepository) Save(ctx context.Context, attachment *entity.Attachment) (int64, error) {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(attachment)
}
func (r attachmentTxRepository) SelectByID(ctx context.Context, id int64) (*entity.Attachment, error) {
	_ = ctx
	return r.repo.selectByID(id)
}
func (r attachmentTxRepository) SelectByUUID(ctx context.Context, attachmentUUID string) (*entity.Attachment, error) {
	_ = ctx
	return r.repo.selectByUUID(attachmentUUID)
}
func (r attachmentTxRepository) SelectByPostID(ctx context.Context, postID int64) ([]*entity.Attachment, error) {
	_ = ctx
	return r.repo.selectByPostID(postID)
}
func (r attachmentTxRepository) SelectCleanupCandidatesBefore(ctx context.Context, cutoff time.Time, limit int) ([]*entity.Attachment, error) {
	_ = ctx
	return r.repo.selectCleanupCandidatesBefore(cutoff, limit)
}
func (r attachmentTxRepository) Update(ctx context.Context, attachment *entity.Attachment) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(attachment)
}
func (r attachmentTxRepository) Delete(ctx context.Context, id int64) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.delete(id)
}

func (r reportTxRepository) Save(ctx context.Context, report *entity.Report) (int64, error) {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.save(report)
}

func (r reportTxRepository) SelectByID(ctx context.Context, id int64) (*entity.Report, error) {
	_ = ctx
	return r.repo.selectByID(id)
}

func (r reportTxRepository) SelectByReporterAndTarget(ctx context.Context, reporterUserID int64, targetType entity.ReportTargetType, targetID int64) (*entity.Report, error) {
	_ = ctx
	return r.repo.selectByReporterAndTarget(reporterUserID, targetType, targetID)
}

func (r reportTxRepository) SelectList(ctx context.Context, status *entity.ReportStatus, limit int, lastID int64) ([]*entity.Report, error) {
	_ = ctx
	return r.repo.selectList(status, limit, lastID)
}
func (r reportTxRepository) ExistsByReporterUserID(ctx context.Context, reporterUserID int64) (bool, error) {
	_ = ctx
	return r.repo.ExistsByReporterUserID(ctx, reporterUserID)
}

func (r reportTxRepository) Update(ctx context.Context, report *entity.Report) error {
	_ = ctx
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.update(report)
}

func (r outboxTxAppender) Append(messages ...port.OutboxMessage) error {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	return r.repo.append(messages...)
}
