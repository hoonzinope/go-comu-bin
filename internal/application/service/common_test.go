package service

import (
	"errors"
	"io"
	"log/slog"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	noopCache "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/noop"
	eventInProcess "github.com/hoonzinope/go-comu-bin/internal/infrastructure/event/inprocess"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/logging"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
)

type testRepositories struct {
	user       port.UserRepository
	board      port.BoardRepository
	post       port.PostRepository
	tag        port.TagRepository
	postTag    port.PostTagRepository
	comment    port.CommentRepository
	reaction   port.ReactionRepository
	attachment port.AttachmentRepository
	unitOfWork port.UnitOfWork
}

func newTestRepositories() testRepositories {
	userRepository := inmemory.NewUserRepository()
	boardRepository := inmemory.NewBoardRepository()
	tagRepository := inmemory.NewTagRepository()
	postTagRepository := inmemory.NewPostTagRepository()
	postRepository := inmemory.NewPostRepository(tagRepository, postTagRepository)
	commentRepository := inmemory.NewCommentRepository()
	reactionRepository := inmemory.NewReactionRepository()
	attachmentRepository := inmemory.NewAttachmentRepository()
	return testRepositories{
		user:       userRepository,
		board:      boardRepository,
		post:       postRepository,
		tag:        tagRepository,
		postTag:    postTagRepository,
		comment:    commentRepository,
		reaction:   reactionRepository,
		attachment: attachmentRepository,
		unitOfWork: inmemory.NewUnitOfWork(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, commentRepository, reactionRepository, attachmentRepository),
	}
}

func newTestCache() port.Cache {
	return noopCache.NewNoopCache()
}

func newTestCachePolicy() appcache.Policy {
	return appcache.Policy{
		ListTTLSeconds:   30,
		DetailTTLSeconds: 30,
	}
}

func newTestAuthorizationPolicy() policy.AuthorizationPolicy {
	return policy.NewRoleAuthorizationPolicy()
}

func newTestPostService(repositories testRepositories, cache port.Cache) *PostService {
	eventPublisher := newTestEventPublisher(cache)
	return NewPostServiceWithPublisher(
		repositories.user,
		repositories.board,
		repositories.post,
		repositories.tag,
		repositories.postTag,
		repositories.attachment,
		repositories.comment,
		repositories.reaction,
		repositories.unitOfWork,
		cache,
		eventPublisher,
		newTestCachePolicy(),
		newTestAuthorizationPolicy(),
	)
}

func newTestEventPublisher(cache port.Cache) port.EventPublisher {
	logger := logging.NewSlogLogger(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	bus := eventInProcess.NewEventBus(logger)
	handler := appevent.NewCacheInvalidationHandler(cache, logger)
	bus.Subscribe(appevent.EventNameBoardChanged, handler)
	bus.Subscribe(appevent.EventNamePostChanged, handler)
	bus.Subscribe(appevent.EventNameCommentChanged, handler)
	bus.Subscribe(appevent.EventNameReactionChanged, handler)
	bus.Subscribe(appevent.EventNameAttachmentChanged, handler)
	return bus
}

func newTestPasswordHasher() port.PasswordHasher {
	return auth.NewBcryptPasswordHasher(4)
}

func seedUser(userRepository port.UserRepository, name, password, role string) int64 {
	var user *entity.User
	if role == "admin" {
		user = entity.NewAdmin(name, password)
	} else {
		user = entity.NewUser(name, password)
	}
	id, _ := userRepository.Save(user)
	return id
}

func seedBoard(boardRepository port.BoardRepository, name, description string) int64 {
	board := entity.NewBoard(name, description)
	id, _ := boardRepository.Save(board)
	return id
}

func seedPost(postRepository port.PostRepository, authorID, boardID int64, title, content string) int64 {
	post := entity.NewPost(title, content, authorID, boardID)
	id, _ := postRepository.Save(post)
	return id
}

func seedDraftPost(postRepository port.PostRepository, authorID, boardID int64, title, content string) int64 {
	post := entity.NewDraftPost(title, content, authorID, boardID)
	id, _ := postRepository.Save(post)
	return id
}

func seedComment(commentRepository port.CommentRepository, authorID, postID int64, content string) int64 {
	comment := entity.NewComment(content, authorID, postID, nil)
	id, _ := commentRepository.Save(comment)
	return id
}

func seedCommentWithParent(commentRepository port.CommentRepository, authorID, postID int64, content string, parentID *int64) int64 {
	comment := entity.NewComment(content, authorID, postID, parentID)
	id, _ := commentRepository.Save(comment)
	return id
}

type errorCache struct {
	getErr             error
	setErr             error
	setWithTTLErr      error
	deleteErr          error
	deleteByPrefixErr  error
	getOrSetWithTTLErr error
}

func (c *errorCache) Get(key string) (interface{}, bool, error) {
	return nil, false, c.getErr
}

func (c *errorCache) Set(key string, value interface{}) error {
	return c.setErr
}

func (c *errorCache) SetWithTTL(key string, value interface{}, ttlSeconds int) error {
	if c.setWithTTLErr != nil {
		return c.setWithTTLErr
	}
	if c.setErr != nil {
		return c.setErr
	}
	return nil
}

func (c *errorCache) Delete(key string) error {
	return c.deleteErr
}

func (c *errorCache) DeleteByPrefix(prefix string) (int, error) {
	return 0, c.deleteByPrefixErr
}

func (c *errorCache) GetOrSetWithTTL(key string, ttlSeconds int, loader func() (interface{}, error)) (interface{}, error) {
	if c.getOrSetWithTTLErr != nil {
		return nil, customError.WrapCache("get or set cache", c.getOrSetWithTTLErr)
	}
	if c.getErr != nil {
		return nil, customError.WrapCache("get cache", c.getErr)
	}
	return loader()
}

func newCacheFailure(err error) error {
	if err != nil {
		return err
	}
	return errors.New("cache unavailable")
}

func assertCacheFailure(t interface {
	Errorf(format string, args ...interface{})
	Helper()
}, err error) {
	t.Helper()
	if !errors.Is(err, customError.ErrCacheFailure) {
		t.Errorf("expected cache failure, got %v", err)
	}
}
