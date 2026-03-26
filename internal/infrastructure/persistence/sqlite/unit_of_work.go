package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hoonzinope/go-comu-bin/internal/application/port"
)

var _ port.UnitOfWork = (*UnitOfWork)(nil)

type UnitOfWork struct {
	db                     *sql.DB
	boardRepository        port.BoardRepository
	postRepository         port.PostRepository
	tagRepository          port.TagRepository
	postTagRepository      port.PostTagRepository
	commentRepository      port.CommentRepository
	reactionRepository     port.ReactionRepository
	attachmentRepository   port.AttachmentRepository
	reportRepository       port.ReportRepository
	notificationRepository port.NotificationRepository
	emailVerificationRepo  *EmailVerificationTokenRepository
	passwordResetRepo      *PasswordResetTokenRepository
	outboxRepository       *OutboxRepository
}

func NewUnitOfWork(
	db *sql.DB,
	boardRepository port.BoardRepository,
	postRepository port.PostRepository,
	tagRepository port.TagRepository,
	postTagRepository port.PostTagRepository,
	commentRepository port.CommentRepository,
	reactionRepository port.ReactionRepository,
	attachmentRepository port.AttachmentRepository,
	reportRepository port.ReportRepository,
	notificationRepository port.NotificationRepository,
	emailVerificationRepo *EmailVerificationTokenRepository,
	passwordResetRepo *PasswordResetTokenRepository,
	outboxRepository *OutboxRepository,
) *UnitOfWork {
	return &UnitOfWork{
		db:                     db,
		boardRepository:        boardRepository,
		postRepository:         postRepository,
		tagRepository:          tagRepository,
		postTagRepository:      postTagRepository,
		commentRepository:      commentRepository,
		reactionRepository:     reactionRepository,
		attachmentRepository:   attachmentRepository,
		reportRepository:       reportRepository,
		notificationRepository: notificationRepository,
		emailVerificationRepo:  emailVerificationRepo,
		passwordResetRepo:      passwordResetRepo,
		outboxRepository:       outboxRepository,
	}
}

func (u *UnitOfWork) WithinTransaction(ctx context.Context, fn func(tx port.TxScope) error) error {
	if u == nil || u.db == nil {
		return fmt.Errorf("sqlite unit of work is not initialized")
	}
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	scope := sqliteTxScope{
		ctx:                    ctx,
		userRepository:         NewUserRepository(tx),
		boardRepository:        NewBoardRepository(tx),
		postRepository:         NewPostRepository(tx),
		tagRepository:          NewTagRepository(tx),
		postTagRepository:      NewPostTagRepository(tx),
		commentRepository:      NewCommentRepository(tx),
		reactionRepository:     NewReactionRepository(tx),
		attachmentRepository:   NewAttachmentRepository(tx),
		reportRepository:       NewReportRepository(tx),
		notificationRepository: NewNotificationRepository(tx),
		emailVerificationRepo:  NewEmailVerificationTokenRepository(tx),
		passwordResetRepo:      NewPasswordResetTokenRepository(tx),
		outbox:                 NewOutboxAppender(tx),
	}
	if err := fn(&scope); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite transaction: %w", err)
	}
	for _, hook := range scope.afterCommit {
		if hook == nil {
			continue
		}
		if err := hook(); err != nil {
			return err
		}
	}
	return nil
}

type sqliteTxScope struct {
	ctx                    context.Context
	userRepository         *UserRepository
	boardRepository        port.BoardRepository
	postRepository         port.PostRepository
	tagRepository          port.TagRepository
	postTagRepository      port.PostTagRepository
	commentRepository      port.CommentRepository
	reactionRepository     port.ReactionRepository
	attachmentRepository   port.AttachmentRepository
	reportRepository       port.ReportRepository
	notificationRepository port.NotificationRepository
	emailVerificationRepo  *EmailVerificationTokenRepository
	passwordResetRepo      *PasswordResetTokenRepository
	outbox                 port.OutboxAppender
	afterCommit            []func() error
}

func (s *sqliteTxScope) AfterCommit(fn func() error) {
	if fn == nil {
		return
	}
	s.afterCommit = append(s.afterCommit, fn)
}

func (s *sqliteTxScope) Context() context.Context                    { return s.ctx }
func (s *sqliteTxScope) UserRepository() port.UserRepository         { return s.userRepository }
func (s *sqliteTxScope) BoardRepository() port.BoardRepository       { return s.boardRepository }
func (s *sqliteTxScope) PostRepository() port.PostRepository         { return s.postRepository }
func (s *sqliteTxScope) TagRepository() port.TagRepository           { return s.tagRepository }
func (s *sqliteTxScope) PostTagRepository() port.PostTagRepository   { return s.postTagRepository }
func (s *sqliteTxScope) CommentRepository() port.CommentRepository   { return s.commentRepository }
func (s *sqliteTxScope) ReactionRepository() port.ReactionRepository { return s.reactionRepository }
func (s *sqliteTxScope) AttachmentRepository() port.AttachmentRepository {
	return s.attachmentRepository
}
func (s *sqliteTxScope) ReportRepository() port.ReportRepository { return s.reportRepository }
func (s *sqliteTxScope) NotificationRepository() port.NotificationRepository {
	return s.notificationRepository
}
func (s *sqliteTxScope) EmailVerificationTokenRepository() port.EmailVerificationTokenRepository {
	return s.emailVerificationRepo
}
func (s *sqliteTxScope) PasswordResetTokenRepository() port.PasswordResetTokenRepository {
	return s.passwordResetRepo
}
func (s *sqliteTxScope) Outbox() port.OutboxAppender { return s.outbox }
