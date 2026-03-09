package port

type TxScope interface {
	UserRepository() UserRepository
	BoardRepository() BoardRepository
	PostRepository() PostRepository
	TagRepository() TagRepository
	PostTagRepository() PostTagRepository
	CommentRepository() CommentRepository
	ReactionRepository() ReactionRepository
	AttachmentRepository() AttachmentRepository
}

type UnitOfWork interface {
	WithinTransaction(fn func(tx TxScope) error) error
}
