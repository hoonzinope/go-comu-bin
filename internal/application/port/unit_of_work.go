package port

import "context"

type TxScope interface {
	Context() context.Context
	UserRepository() UserRepository
	BoardRepository() BoardRepository
	PostRepository() PostRepository
	TagRepository() TagRepository
	PostTagRepository() PostTagRepository
	CommentRepository() CommentRepository
	ReactionRepository() ReactionRepository
	AttachmentRepository() AttachmentRepository
	Outbox() OutboxAppender
}

type UnitOfWork interface {
	WithinTransaction(ctx context.Context, fn func(tx TxScope) error) error
}
