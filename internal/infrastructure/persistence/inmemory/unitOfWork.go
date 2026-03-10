package inmemory

import (
	"sync"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.UnitOfWork = (*UnitOfWork)(nil)
var _ port.TxScope = (*txScope)(nil)

type UnitOfWork struct {
	mu                   sync.Mutex
	coordinator          *txCoordinator
	userRepository       *UserRepository
	boardRepository      *BoardRepository
	postRepository       *PostRepository
	tagRepository        *TagRepository
	postTagRepo          *PostTagRepository
	commentRepository    *CommentRepository
	reactionRepository   *ReactionRepository
	attachmentRepository *AttachmentRepository
}

type txScope struct {
	userRepository       port.UserRepository
	boardRepository      port.BoardRepository
	postRepository       port.PostRepository
	tagRepository        port.TagRepository
	postTagRepository    port.PostTagRepository
	commentRepository    port.CommentRepository
	reactionRepository   port.ReactionRepository
	attachmentRepository port.AttachmentRepository
}

func NewUnitOfWork(userRepository *UserRepository, boardRepository *BoardRepository, postRepository *PostRepository, tagRepository *TagRepository, postTagRepo *PostTagRepository, commentRepository *CommentRepository, reactionRepository *ReactionRepository, attachmentRepository *AttachmentRepository) *UnitOfWork {
	coordinator := newTxCoordinator()
	userRepository.attachCoordinator(coordinator)
	boardRepository.attachCoordinator(coordinator)
	postRepository.attachCoordinator(coordinator)
	tagRepository.attachCoordinator(coordinator)
	postTagRepo.attachCoordinator(coordinator)
	commentRepository.attachCoordinator(coordinator)
	reactionRepository.attachCoordinator(coordinator)
	attachmentRepository.attachCoordinator(coordinator)

	return &UnitOfWork{
		coordinator:          coordinator,
		userRepository:       userRepository,
		boardRepository:      boardRepository,
		postRepository:       postRepository,
		tagRepository:        tagRepository,
		postTagRepo:          postTagRepo,
		commentRepository:    commentRepository,
		reactionRepository:   reactionRepository,
		attachmentRepository: attachmentRepository,
	}
}

func (u *UnitOfWork) WithinTransaction(fn func(tx port.TxScope) error) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.coordinator.lock()
	defer u.coordinator.unlock()

	postState := u.postRepository.snapshot()
	userState := u.userRepository.snapshot()
	boardState := u.boardRepository.snapshot()
	tagState := u.tagRepository.snapshot()
	postTagState := u.postTagRepo.snapshot()
	commentState := u.commentRepository.snapshot()
	reactionState := u.reactionRepository.snapshot()
	attachmentState := u.attachmentRepository.snapshot()

	tx := &txScope{
		userRepository:       userTxRepository{repo: u.userRepository},
		boardRepository:      boardTxRepository{repo: u.boardRepository},
		postRepository:       postTxRepository{repo: u.postRepository},
		tagRepository:        tagTxRepository{repo: u.tagRepository},
		postTagRepository:    postTagTxRepository{repo: u.postTagRepo},
		commentRepository:    commentTxRepository{repo: u.commentRepository},
		reactionRepository:   reactionTxRepository{repo: u.reactionRepository},
		attachmentRepository: attachmentTxRepository{repo: u.attachmentRepository},
	}
	if err := fn(tx); err != nil {
		u.userRepository.restore(userState)
		u.boardRepository.restore(boardState)
		u.postRepository.restore(postState)
		u.tagRepository.restore(tagState)
		u.postTagRepo.restore(postTagState)
		u.commentRepository.restore(commentState)
		u.reactionRepository.restore(reactionState)
		u.attachmentRepository.restore(attachmentState)
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

type postTxRepository struct{ repo *PostRepository }

type userTxRepository struct{ repo *UserRepository }

func (r userTxRepository) Save(user *entity.User) (int64, error) {
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
	return r.repo.update(user)
}
func (r userTxRepository) Delete(id int64) error {
	return r.repo.delete(id)
}

type boardTxRepository struct{ repo *BoardRepository }

func (r boardTxRepository) SelectBoardByID(id int64) (*entity.Board, error) {
	return r.repo.selectBoardByID(id)
}
func (r boardTxRepository) SelectBoardList(limit int, lastID int64) ([]*entity.Board, error) {
	return r.repo.selectBoardList(limit, lastID)
}
func (r boardTxRepository) Save(board *entity.Board) (int64, error) {
	return r.repo.save(board)
}
func (r boardTxRepository) Update(board *entity.Board) error {
	return r.repo.update(board)
}
func (r boardTxRepository) Delete(id int64) error {
	return r.repo.delete(id)
}

func (r postTxRepository) Save(post *entity.Post) (int64, error) {
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
	return r.repo.update(post)
}
func (r postTxRepository) Delete(id int64) error {
	return r.repo.delete(id)
}

type tagTxRepository struct{ repo *TagRepository }

func (r tagTxRepository) Save(tag *entity.Tag) (int64, error) {
	return r.repo.save(tag)
}
func (r tagTxRepository) SelectByName(name string) (*entity.Tag, error) {
	return r.repo.selectByName(name)
}
func (r tagTxRepository) SelectByIDs(ids []int64) ([]*entity.Tag, error) {
	return r.repo.selectByIDs(ids)
}

type postTagTxRepository struct{ repo *PostTagRepository }

func (r postTagTxRepository) SelectActiveByPostID(postID int64) ([]*entity.PostTag, error) {
	return r.repo.selectActiveByPostID(postID)
}
func (r postTagTxRepository) SelectActiveByTagID(tagID int64, limit int, lastID int64) ([]*entity.PostTag, error) {
	return r.repo.selectActiveByTagID(tagID, limit, lastID)
}
func (r postTagTxRepository) UpsertActive(postID, tagID int64) error {
	return r.repo.upsertActive(postID, tagID)
}
func (r postTagTxRepository) SoftDelete(postID, tagID int64) error {
	return r.repo.softDelete(postID, tagID)
}
func (r postTagTxRepository) SoftDeleteByPostID(postID int64) error {
	return r.repo.softDeleteByPostID(postID)
}

type commentTxRepository struct{ repo *CommentRepository }

func (r commentTxRepository) Save(comment *entity.Comment) (int64, error) {
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
	return r.repo.update(comment)
}
func (r commentTxRepository) Delete(id int64) error {
	return r.repo.delete(id)
}

type reactionTxRepository struct{ repo *ReactionRepository }

func (r reactionTxRepository) SetUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType, reactionType entity.ReactionType) (*entity.Reaction, bool, bool, error) {
	return r.repo.setUserTargetReaction(userID, targetID, targetType, reactionType)
}
func (r reactionTxRepository) DeleteUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType) (bool, error) {
	return r.repo.deleteUserTargetReaction(userID, targetID, targetType)
}
func (r reactionTxRepository) DeleteByTarget(targetID int64, targetType entity.ReactionTargetType) (int, error) {
	return r.repo.deleteByTarget(targetID, targetType)
}
func (r reactionTxRepository) GetUserTargetReaction(userID, targetID int64, targetType entity.ReactionTargetType) (*entity.Reaction, error) {
	return r.repo.getUserTargetReaction(userID, targetID, targetType)
}
func (r reactionTxRepository) GetByTarget(targetID int64, targetType entity.ReactionTargetType) ([]*entity.Reaction, error) {
	return r.repo.getByTarget(targetID, targetType)
}

type attachmentTxRepository struct{ repo *AttachmentRepository }

func (r attachmentTxRepository) Save(attachment *entity.Attachment) (int64, error) {
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
	return r.repo.update(attachment)
}
func (r attachmentTxRepository) Delete(id int64) error {
	return r.repo.delete(id)
}
