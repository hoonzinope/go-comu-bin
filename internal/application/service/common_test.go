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
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	noopCache "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/noop"
	eventOutbox "github.com/hoonzinope/go-comu-bin/internal/infrastructure/event/outbox"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
	"github.com/stretchr/testify/require"
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
	report     port.ReportRepository
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
	reportRepository := inmemory.NewReportRepository()
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
		report:     reportRepository,
		outbox:     outboxRepository,
		unitOfWork: inmemory.NewUnitOfWork(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, commentRepository, reactionRepository, attachmentRepository, reportRepository, outboxRepository),
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
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
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
	relay.Subscribe(appevent.EventNameReportChanged, handler)
	relayCtx, relayCancel := context.WithCancel(context.Background())
	relay.Start(relayCtx)
	t.Cleanup(func() {
		relayCancel()
		relay.Wait()
	})
	return wrapEventPublisherAsActionDispatcher(eventOutbox.NewPublisher(repositories.outbox, serializer, logger))
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
	id, _ := userRepository.Save(context.Background(), user)
	return id
}

func seedBoard(boardRepository port.BoardRepository, name, description string) int64 {
	board := entity.NewBoard(name, description)
	id, _ := boardRepository.Save(context.Background(), board)
	return id
}

func seedPost(postRepository port.PostRepository, authorID, boardID int64, title, content string) int64 {
	post := entity.NewPost(title, content, authorID, boardID)
	id, _ := postRepository.Save(context.Background(), post)
	return id
}

func seedDraftPost(postRepository port.PostRepository, authorID, boardID int64, title, content string) int64 {
	post := entity.NewDraftPost(title, content, authorID, boardID)
	id, _ := postRepository.Save(context.Background(), post)
	return id
}

func seedComment(commentRepository port.CommentRepository, authorID, postID int64, content string) int64 {
	comment := entity.NewComment(content, authorID, postID, nil)
	id, _ := commentRepository.Save(context.Background(), comment)
	return id
}

func seedCommentWithParent(commentRepository port.CommentRepository, authorID, postID int64, content string, parentID *int64) int64 {
	comment := entity.NewComment(content, authorID, postID, parentID)
	id, _ := commentRepository.Save(context.Background(), comment)
	return id
}

func mustBoardUUID(t testing.TB, repo port.BoardRepository, boardID int64) string {
	t.Helper()
	board, err := repo.SelectBoardByID(context.Background(), boardID)
	require.NoError(t, err)
	require.NotNil(t, board)
	return board.UUID
}

func mustPostUUID(t testing.TB, repo port.PostRepository, postID int64) string {
	t.Helper()
	post, err := repo.SelectPostByIDIncludingUnpublished(context.Background(), postID)
	require.NoError(t, err)
	require.NotNil(t, post)
	return post.UUID
}

func mustCommentUUID(t testing.TB, repo port.CommentRepository, commentID int64) string {
	t.Helper()
	comment, err := repo.SelectCommentByID(context.Background(), commentID)
	require.NoError(t, err)
	require.NotNil(t, comment)
	return comment.UUID
}

func mustAttachmentUUID(t testing.TB, repo port.AttachmentRepository, attachmentID int64) string {
	t.Helper()
	attachment, err := repo.SelectByID(context.Background(), attachmentID)
	require.NoError(t, err)
	require.NotNil(t, attachment)
	return attachment.UUID
}

type errorCache struct {
	getErr             error
	setErr             error
	setWithTTLErr      error
	deleteErr          error
	deleteByPrefixErr  error
	getOrSetWithTTLErr error
}

func (c *errorCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	_ = ctx
	_ = key
	return nil, false, c.getErr
}

func (c *errorCache) Set(ctx context.Context, key string, value interface{}) error {
	_ = ctx
	_ = key
	_ = value
	return c.setErr
}

func (c *errorCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttlSeconds int) error {
	_ = ctx
	_ = key
	_ = value
	_ = ttlSeconds
	if c.setWithTTLErr != nil {
		return c.setWithTTLErr
	}
	if c.setErr != nil {
		return c.setErr
	}
	return nil
}

func (c *errorCache) Delete(ctx context.Context, key string) error {
	_ = ctx
	_ = key
	return c.deleteErr
}

func (c *errorCache) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	_ = ctx
	_ = prefix
	return 0, c.deleteByPrefixErr
}

func (c *errorCache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	_ = ctx
	_ = key
	_ = ttlSeconds
	if c.getOrSetWithTTLErr != nil {
		return nil, customerror.WrapCache("get or set cache", c.getOrSetWithTTLErr)
	}
	if c.getErr != nil {
		return nil, customerror.WrapCache("get cache", c.getErr)
	}
	return loader(ctx)
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
	if !errors.Is(err, customerror.ErrCacheFailure) {
		t.Errorf("expected cache failure, got %v", err)
	}
}

type hookCache struct {
	onLoad func()
}

func (c *hookCache) Get(context.Context, string) (interface{}, bool, error) {
	return nil, false, nil
}

func (c *hookCache) Set(context.Context, string, interface{}) error {
	return nil
}

func (c *hookCache) SetWithTTL(context.Context, string, interface{}, int) error {
	return nil
}

func (c *hookCache) Delete(context.Context, string) error {
	return nil
}

func (c *hookCache) DeleteByPrefix(context.Context, string) (int, error) {
	return 0, nil
}

func (c *hookCache) GetOrSetWithTTL(ctx context.Context, key string, ttlSeconds int, loader func(context.Context) (interface{}, error)) (interface{}, error) {
	_ = key
	_ = ttlSeconds
	if c.onLoad != nil {
		c.onLoad()
	}
	return loader(ctx)
}
