package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	noopCache "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/noop"
	eventOutbox "github.com/hoonzinope/go-comu-bin/internal/infrastructure/event/outbox"
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
	outbox     port.OutboxStore
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
	outboxRepository := inmemory.NewOutboxRepository()
	return testRepositories{
		user:       userRepository,
		board:      boardRepository,
		post:       postRepository,
		tag:        tagRepository,
		postTag:    postTagRepository,
		comment:    commentRepository,
		reaction:   reactionRepository,
		attachment: attachmentRepository,
		outbox:     outboxRepository,
		unitOfWork: inmemory.NewUnitOfWork(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, commentRepository, reactionRepository, attachmentRepository, outboxRepository),
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

func newTestPostService(t testing.TB, repositories testRepositories, cache port.Cache) *PostService {
	t.Helper()
	actionDispatcher := newTestActionDispatcher(t, repositories, cache)
	return NewPostServiceWithActionDispatcher(
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
		actionDispatcher,
		newTestCachePolicy(),
		newTestAuthorizationPolicy(),
	)
}

func newTestActionDispatcher(t testing.TB, repositories testRepositories, cache port.Cache) port.ActionHookDispatcher {
	t.Helper()
	logger := logging.NewSlogLogger(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	serializer := appevent.NewJSONEventSerializer()
	relay := eventOutbox.NewRelay(repositories.outbox, serializer, logger, eventOutbox.RelayConfig{
		WorkerCount:  1,
		BatchSize:    64,
		PollInterval: time.Millisecond,
		MaxAttempts:  5,
		BaseBackoff:  time.Millisecond,
	})
	handler := appevent.NewCacheInvalidationHandler(cache, logger)
	relay.Subscribe(appevent.EventNameBoardChanged, handler)
	relay.Subscribe(appevent.EventNamePostChanged, handler)
	relay.Subscribe(appevent.EventNameCommentChanged, handler)
	relay.Subscribe(appevent.EventNameReactionChanged, handler)
	relay.Subscribe(appevent.EventNameAttachmentChanged, handler)
	relayCtx, relayCancel := context.WithCancel(context.Background())
	relay.Start(relayCtx)
	t.Cleanup(func() {
		relayCancel()
		relay.Wait()
	})
	return wrapEventPublisherAsActionDispatcher(eventOutbox.NewPublisher(repositories.outbox, serializer, logger))
}

// Deprecated: use newTestActionDispatcher.
func newTestEventPublisher(t testing.TB, repositories testRepositories, cache port.Cache) port.EventPublisher {
	t.Helper()
	logger := logging.NewSlogLogger(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	serializer := appevent.NewJSONEventSerializer()
	relay := eventOutbox.NewRelay(repositories.outbox, serializer, logger, eventOutbox.RelayConfig{
		WorkerCount:  1,
		BatchSize:    64,
		PollInterval: time.Millisecond,
		MaxAttempts:  5,
		BaseBackoff:  time.Millisecond,
	})
	handler := appevent.NewCacheInvalidationHandler(cache, logger)
	relay.Subscribe(appevent.EventNameBoardChanged, handler)
	relay.Subscribe(appevent.EventNamePostChanged, handler)
	relay.Subscribe(appevent.EventNameCommentChanged, handler)
	relay.Subscribe(appevent.EventNameReactionChanged, handler)
	relay.Subscribe(appevent.EventNameAttachmentChanged, handler)
	relayCtx, relayCancel := context.WithCancel(context.Background())
	relay.Start(relayCtx)
	t.Cleanup(func() {
		relayCancel()
		relay.Wait()
	})
	return eventOutbox.NewPublisher(repositories.outbox, serializer, logger)
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
