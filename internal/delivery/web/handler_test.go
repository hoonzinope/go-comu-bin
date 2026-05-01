package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testSessionUseCase struct {
	validateErr error
	loginErr    error
	logoutErr   error

	validateCalls   int
	loginCalls      int
	logoutCalls     int
	issueGuestCalls int
}

func (s *testSessionUseCase) ValidateTokenToId(ctx context.Context, token string) (int64, error) {
	_ = ctx
	s.validateCalls++
	if s.validateErr != nil {
		return 0, s.validateErr
	}
	if strings.TrimSpace(token) == "" {
		return 0, context.Canceled
	}
	if token == "admin-token" {
		return 1, nil
	}
	return 2, nil
}

func (s *testSessionUseCase) Login(ctx context.Context, username, password string) (string, error) {
	_, _, _ = ctx, username, password
	s.loginCalls++
	if s.loginErr != nil {
		return "", s.loginErr
	}
	return "token", nil
}

func (s *testSessionUseCase) IssueGuestToken(ctx context.Context) (string, error) {
	_ = ctx
	s.issueGuestCalls++
	return "guest-token", nil
}

func (s *testSessionUseCase) RotateToken(ctx context.Context, userID int64, currentToken string) (string, error) {
	_, _, _ = ctx, userID, currentToken
	return "rotated-token", nil
}

func (s *testSessionUseCase) Logout(ctx context.Context, token string) error {
	_, _ = ctx, token
	s.logoutCalls++
	if s.logoutErr != nil {
		return s.logoutErr
	}
	return nil
}

func (s *testSessionUseCase) InvalidateUserSessions(ctx context.Context, userID int64) error {
	_, _ = ctx, userID
	return nil
}

type testUserUseCase struct{}

func (u *testUserUseCase) GetMe(ctx context.Context, userID int64) (*model.User, error) {
	_ = ctx
	user := &model.User{ID: userID, UUID: "user-uuid", Name: "Alice", Email: "alice@example.com", Role: "user", Status: entity.UserStatusActive, CreatedAt: time.Unix(10, 0), UpdatedAt: time.Unix(10, 0)}
	if userID == 1 {
		user.Role = "admin"
		user.Name = "Admin"
	} else if userID == 2 {
		user.Guest = true
		user.Name = "Guest"
		user.Email = ""
	}
	return user, nil
}

func (u *testUserUseCase) DeleteMe(ctx context.Context, userID int64, password string) error {
	_, _, _ = ctx, userID, password
	return nil
}

func (u *testUserUseCase) SignUp(ctx context.Context, username, email, password string) (string, error) {
	_, _, _ = ctx, username, email
	_ = password
	return "signed-up", nil
}

func (u *testUserUseCase) IssueGuestAccount(ctx context.Context) (int64, error) {
	_ = ctx
	return 2, nil
}

func (u *testUserUseCase) UpgradeGuest(ctx context.Context, userID int64, username, email, password string) error {
	_, _, _, _ = ctx, userID, username, email
	_ = password
	return nil
}

func (u *testUserUseCase) GetUserSuspension(ctx context.Context, adminID int64, targetUserUUID string) (*model.UserSuspension, error) {
	_, _, _ = ctx, adminID, targetUserUUID
	return &model.UserSuspension{UserUUID: targetUserUUID, Status: entity.UserStatusActive}, nil
}

func (u *testUserUseCase) SuspendUser(ctx context.Context, adminID int64, targetUserUUID, reason string, duration model.SuspensionDuration) error {
	_, _, _, _ = ctx, adminID, targetUserUUID, reason
	_ = duration
	return nil
}

func (u *testUserUseCase) UnsuspendUser(ctx context.Context, adminID int64, targetUserUUID string) error {
	_, _, _ = ctx, adminID, targetUserUUID
	return nil
}

type testBoardUseCase struct {
	boards    *model.BoardList
	allBoards *model.BoardList
}

func (b *testBoardUseCase) GetBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error) {
	_, _, _ = ctx, limit, cursor
	if b != nil && b.boards != nil {
		return b.boards, nil
	}
	return &model.BoardList{
		Boards: []model.Board{
			{UUID: "general-uuid", Name: "General", Description: "Visible board"},
		},
	}, nil
}

func (b *testBoardUseCase) GetAllBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error) {
	_, _, _ = ctx, limit, cursor
	if b != nil && b.allBoards != nil {
		return b.allBoards, nil
	}
	return &model.BoardList{
		Boards: []model.Board{
			{UUID: "general-uuid", Name: "General", Description: "Visible board", Hidden: false},
			{UUID: "hidden-uuid", Name: "Hidden", Description: "Hidden board", Hidden: true},
		},
	}, nil
}

func (b *testBoardUseCase) CreateBoard(ctx context.Context, userID int64, name, description string) (string, error) {
	_, _, _, _ = ctx, userID, name, description
	return "new-board-uuid", nil
}

func (b *testBoardUseCase) UpdateBoard(ctx context.Context, boardUUID string, userID int64, name, description string) error {
	_, _, _, _, _ = ctx, boardUUID, userID, name, description
	return nil
}

func (b *testBoardUseCase) DeleteBoard(ctx context.Context, boardUUID string, userID int64) error {
	_, _, _ = ctx, boardUUID, userID
	return nil
}

func (b *testBoardUseCase) SetBoardVisibility(ctx context.Context, boardUUID string, userID int64, hidden bool) error {
	_, _, _ = ctx, boardUUID, userID
	_ = hidden
	return nil
}

type testBoardUseCaseEmpty struct{}

func (b *testBoardUseCaseEmpty) GetBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error) {
	_, _, _ = ctx, limit, cursor
	return &model.BoardList{}, nil
}

func (b *testBoardUseCaseEmpty) GetAllBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error) {
	_, _, _ = ctx, limit, cursor
	return &model.BoardList{}, nil
}

func (b *testBoardUseCaseEmpty) CreateBoard(ctx context.Context, userID int64, name, description string) (string, error) {
	_, _, _, _ = ctx, userID, name, description
	return "new-board-uuid", nil
}

func (b *testBoardUseCaseEmpty) UpdateBoard(ctx context.Context, boardUUID string, userID int64, name, description string) error {
	_, _, _, _, _ = ctx, boardUUID, userID, name, description
	return nil
}

func (b *testBoardUseCaseEmpty) DeleteBoard(ctx context.Context, boardUUID string, userID int64) error {
	_, _, _ = ctx, boardUUID, userID
	return nil
}

func (b *testBoardUseCaseEmpty) SetBoardVisibility(ctx context.Context, boardUUID string, userID int64, hidden bool) error {
	_, _, _ = ctx, boardUUID, userID
	_ = hidden
	return nil
}

type testReactionUseCase struct{}

func (r *testReactionUseCase) SetReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error) {
	_, _, _, _, _ = ctx, userID, targetUUID, targetType, reactionType
	return true, nil
}

func (r *testReactionUseCase) DeleteReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error {
	_, _, _, _ = ctx, userID, targetUUID, targetType
	return nil
}

type testPostUseCase struct {
	boardList  *model.PostList
	draftList  *model.PostList
	feedList   *model.PostList
	tagList    *model.PostList
	searchList *model.PostList
}

func (p *testPostUseCase) CreatePost(ctx context.Context, title, content string, tags []string, mentionedUsernames []string, authorID int64, boardUUID string) (string, error) {
	_, _, _, _, _, _ = ctx, title, content, tags, mentionedUsernames, authorID
	_ = boardUUID
	return "post-uuid", nil
}

func (p *testPostUseCase) CreateDraftPost(ctx context.Context, title, content string, tags []string, mentionedUsernames []string, authorID int64, boardUUID string) (string, error) {
	_, _, _, _, _, _ = ctx, title, content, tags, mentionedUsernames, authorID
	_ = boardUUID
	return "draft-uuid", nil
}

func (p *testPostUseCase) GetPostsList(ctx context.Context, boardUUID string, sort string, window string, limit int, cursor string) (*model.PostList, error) {
	_, _, _, _, _, _ = ctx, boardUUID, sort, window, limit, cursor
	if p != nil && p.boardList != nil {
		return p.boardList, nil
	}
	return &model.PostList{}, nil
}

func (p *testPostUseCase) GetMyDraftPosts(ctx context.Context, authorID int64, limit int, cursor string) (*model.PostList, error) {
	_, _, _, _ = ctx, authorID, limit, cursor
	if p != nil && p.draftList != nil {
		return p.draftList, nil
	}
	return &model.PostList{
		Posts: []model.Post{
			{UUID: "draft-uuid", Title: "Draft title", Content: "Draft body", BoardUUID: "general-uuid", AuthorUUID: "user-uuid", CreatedAt: time.Unix(20, 0), UpdatedAt: time.Unix(20, 0)},
		},
	}, nil
}

func (p *testPostUseCase) GetFeed(ctx context.Context, sort string, window string, limit int, cursor string) (*model.PostList, error) {
	_, _, _, _, _ = ctx, sort, window, limit, cursor
	if p != nil && p.feedList != nil {
		return p.feedList, nil
	}
	return &model.PostList{
		Posts: []model.Post{
			{UUID: "post-uuid", Title: "Feed title", Content: "Feed body", BoardUUID: "general-uuid", AuthorUUID: "user-uuid", CreatedAt: time.Unix(10, 0), UpdatedAt: time.Unix(10, 0)},
		},
	}, nil
}

func (p *testPostUseCase) SearchPosts(ctx context.Context, query string, sort string, window string, limit int, cursor string) (*model.PostList, error) {
	_, _, _, _, _ = ctx, query, sort, window, limit
	_ = cursor
	if p != nil && p.searchList != nil {
		return p.searchList, nil
	}
	return &model.PostList{}, nil
}

func (p *testPostUseCase) GetPostsByTag(ctx context.Context, tagName string, sort string, window string, limit int, cursor string) (*model.PostList, error) {
	_, _, _, _, _ = ctx, tagName, sort, window, limit
	_ = cursor
	if p != nil && p.tagList != nil {
		return p.tagList, nil
	}
	return &model.PostList{}, nil
}

func (p *testPostUseCase) GetPostDetail(ctx context.Context, postUUID string) (*model.PostDetail, error) {
	_, _ = ctx, postUUID
	return &model.PostDetail{
		Post: &model.Post{UUID: "post-uuid", Title: "Feed title", Summary: "Lead summary sentence.", Content: "Lead summary sentence.\n\nFeed body with <script>alert(1)</script> and [docs](https://example.com)", BoardUUID: "general-uuid", AuthorUUID: "user-uuid", CreatedAt: time.Unix(10, 0), UpdatedAt: time.Unix(10, 0)},
		Tags: []model.Tag{
			{Name: "roadmap"},
			{Name: "ui"},
		},
		Attachments: []model.Attachment{
			{UUID: "attach-uuid", FileName: "mock.png", ContentType: "image/png", SizeBytes: 2048},
		},
		Comments: []*model.CommentDetail{
			{
				Comment: &model.Comment{UUID: "comment-uuid", Content: "Looks good.", AuthorUUID: "user-uuid", CreatedAt: time.Unix(15, 0)},
			},
		},
		CommentsHasMore: false,
		Reactions: []model.Reaction{
			{ID: 1, TargetUUID: "post-uuid", UserUUID: "user-uuid"},
		},
	}, nil
}

func (p *testPostUseCase) GetDraftPost(ctx context.Context, postUUID string, userID int64) (*model.PostDetail, error) {
	_, _ = ctx, postUUID
	_ = userID
	return &model.PostDetail{
		Post: &model.Post{UUID: "draft-uuid", Title: "Draft title", Content: "Draft body", BoardUUID: "general-uuid", AuthorUUID: "user-uuid", CreatedAt: time.Unix(20, 0), UpdatedAt: time.Unix(20, 0)},
		Tags: []model.Tag{{Name: "draft"}},
	}, nil
}

func (p *testPostUseCase) PublishPost(ctx context.Context, postUUID string, authorID int64) error {
	_, _, _ = ctx, postUUID, authorID
	return nil
}

func (p *testPostUseCase) UpdatePost(ctx context.Context, postUUID string, authorID int64, title, content string, tags []string) error {
	_, _, _, _, _, _ = ctx, postUUID, authorID, title, content, tags
	return nil
}

func (p *testPostUseCase) DeletePost(ctx context.Context, postUUID string, authorID int64) error {
	_, _, _ = ctx, postUUID, authorID
	return nil
}

type testCommentUseCase struct {
	comments *model.CommentList
}

func (c *testCommentUseCase) CreateComment(ctx context.Context, content string, mentionedUsernames []string, authorID int64, postUUID string, parentUUID *string) (string, error) {
	_, _, _, _, _ = ctx, content, mentionedUsernames, authorID, postUUID
	_ = parentUUID
	return "comment-uuid", nil
}

func (c *testCommentUseCase) GetCommentsByPost(ctx context.Context, postUUID string, limit int, cursor string) (*model.CommentList, error) {
	_, _, _ = ctx, postUUID, limit
	_ = cursor
	if c != nil && c.comments != nil {
		return c.comments, nil
	}
	return &model.CommentList{
		Comments: []model.Comment{
			{UUID: "comment-uuid", Content: "Looks good.", AuthorUUID: "user-uuid", PostUUID: "post-uuid", CreatedAt: time.Unix(15, 0)},
		},
	}, nil
}

func (c *testCommentUseCase) UpdateComment(ctx context.Context, commentUUID string, authorID int64, content string) error {
	_, _, _ = ctx, commentUUID, authorID
	_ = content
	return nil
}

func (c *testCommentUseCase) DeleteComment(ctx context.Context, commentUUID string, authorID int64) error {
	_, _, _ = ctx, commentUUID, authorID
	return nil
}

type testNotificationUseCase struct {
	notifications *model.NotificationList
}

func (n *testNotificationUseCase) GetMyNotifications(ctx context.Context, userID int64, limit int, cursor string) (*model.NotificationList, error) {
	_, _, _ = ctx, userID, limit
	_ = cursor
	if n != nil && n.notifications != nil {
		return n.notifications, nil
	}
	return &model.NotificationList{}, nil
}

func (n *testNotificationUseCase) GetMyUnreadNotificationCount(ctx context.Context, userID int64) (int, error) {
	_, _ = ctx, userID
	return 0, nil
}

func (n *testNotificationUseCase) MarkMyNotificationRead(ctx context.Context, userID int64, notificationUUID string) error {
	_, _, _ = ctx, userID, notificationUUID
	return nil
}

func (n *testNotificationUseCase) MarkAllMyNotificationsRead(ctx context.Context, userID int64) error {
	_, _ = ctx, userID
	return nil
}

type testReportUseCase struct {
	reports *model.ReportList
}

func (r *testReportUseCase) CreateReport(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUID string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error) {
	_, _, _, _, _ = ctx, reporterUserID, targetType, targetUUID, reasonCode
	_ = reasonDetail
	return 1, nil
}

func (r *testReportUseCase) GetReports(ctx context.Context, adminID int64, status *model.ReportStatus, limit int, lastID int64) (*model.ReportList, error) {
	_, _, _, _, _ = ctx, adminID, status, limit, lastID
	if r != nil && r.reports != nil {
		return r.reports, nil
	}
	now := time.Unix(30, 0)
	resolver := "admin-uuid"
	return &model.ReportList{
		Reports: []model.Report{
			{
				ID:             7,
				TargetType:     "post",
				TargetUUID:     "post-uuid",
				ReporterUUID:   "reporter-uuid",
				ReasonCode:     "spam",
				ReasonDetail:   "Looks automated.",
				Status:         "pending",
				ResolutionNote: "",
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				ID:             8,
				TargetType:     "comment",
				TargetUUID:     "comment-uuid",
				ReporterUUID:   "reporter-uuid",
				ReasonCode:     "abuse",
				ReasonDetail:   "Harassing language.",
				Status:         "accepted",
				ResolutionNote: "Removed",
				ResolvedByUUID: &resolver,
				ResolvedAt:     &now,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}, nil
}

func (r *testReportUseCase) ResolveReport(ctx context.Context, adminID, reportID int64, status model.ReportStatus, resolutionNote string) error {
	_, _, _, _ = ctx, adminID, reportID, status
	_ = resolutionNote
	return nil
}

type testOutboxAdminUseCase struct {
	outbox *model.OutboxDeadMessageList
}

func (o *testOutboxAdminUseCase) GetDeadMessages(ctx context.Context, adminID int64, limit int, lastID string) (*model.OutboxDeadMessageList, error) {
	_, _, _, _ = ctx, adminID, limit, lastID
	if o != nil && o.outbox != nil {
		return o.outbox, nil
	}
	now := time.Unix(40, 0)
	return &model.OutboxDeadMessageList{
		Messages: []model.OutboxDeadMessage{
			{ID: "dead-1", EventName: "post.changed", AttemptCount: 3, LastError: "timeout talking to queue", OccurredAt: now, NextAttemptAt: now.Add(time.Minute)},
			{ID: "dead-2", EventName: "report.changed", AttemptCount: 1, LastError: "payload rejected", OccurredAt: now.Add(time.Minute), NextAttemptAt: now.Add(2 * time.Minute)},
		},
	}, nil
}

func (o *testOutboxAdminUseCase) RequeueDeadMessage(ctx context.Context, adminID int64, messageID string) error {
	_, _, _ = ctx, adminID, messageID
	return nil
}

func (o *testOutboxAdminUseCase) DiscardDeadMessage(ctx context.Context, adminID int64, messageID string) error {
	_, _, _ = ctx, adminID, messageID
	return nil
}

type testAccountUseCase struct{}

func (a *testAccountUseCase) DeleteMyAccount(ctx context.Context, userID int64, password string) error {
	_, _, _ = ctx, userID, password
	return nil
}

func (a *testAccountUseCase) UpgradeGuestAccount(ctx context.Context, userID int64, currentToken, username, email, password string) (string, error) {
	_, _, _, _, _ = ctx, userID, currentToken, username, email
	_ = password
	return "upgraded-token", nil
}

func (a *testAccountUseCase) RequestEmailVerification(ctx context.Context, userID int64) error {
	_, _ = ctx, userID
	return nil
}

func (a *testAccountUseCase) ConfirmEmailVerification(ctx context.Context, token string) error {
	_, _ = ctx, token
	return nil
}

func (a *testAccountUseCase) RequestPasswordReset(ctx context.Context, email string) error {
	_ = ctx
	_ = email
	return nil
}

func (a *testAccountUseCase) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	_, _ = ctx, token
	_ = newPassword
	return nil
}

func newTestWebHandler() *Handler {
	return newTestWebHandlerWithSession(&testSessionUseCase{})
}

func newTestWebHandlerWithSession(session *testSessionUseCase) *Handler {
	if session == nil {
		session = &testSessionUseCase{}
	}
	h, err := newTestHandlerWithDependencies(session, &testBoardUseCase{}, &testPostUseCase{}, &testCommentUseCase{}, &testNotificationUseCase{}, &testReportUseCase{}, &testOutboxAdminUseCase{})
	if err != nil {
		panic(err)
	}
	return h
}

func newTestHandlerWithDependencies(session *testSessionUseCase, boardUseCase *testBoardUseCase, postUseCase *testPostUseCase, commentUseCase *testCommentUseCase, notificationUseCase *testNotificationUseCase, reportUseCase *testReportUseCase, outboxUseCase *testOutboxAdminUseCase) (*Handler, error) {
	if session == nil {
		session = &testSessionUseCase{}
	}
	if boardUseCase == nil {
		boardUseCase = &testBoardUseCase{}
	}
	if postUseCase == nil {
		postUseCase = &testPostUseCase{}
	}
	if commentUseCase == nil {
		commentUseCase = &testCommentUseCase{}
	}
	if notificationUseCase == nil {
		notificationUseCase = &testNotificationUseCase{}
	}
	if reportUseCase == nil {
		reportUseCase = &testReportUseCase{}
	}
	if outboxUseCase == nil {
		outboxUseCase = &testOutboxAdminUseCase{}
	}
	return NewHandler(Dependencies{
		AccountUseCase:      &testAccountUseCase{},
		SessionUseCase:      session,
		UserUseCase:         &testUserUseCase{},
		BoardUseCase:        boardUseCase,
		PostUseCase:         postUseCase,
		CommentUseCase:      commentUseCase,
		NotificationUseCase: notificationUseCase,
		ReportUseCase:       reportUseCase,
		OutboxAdminUseCase:  outboxUseCase,
		ReactionUseCase:     &testReactionUseCase{},
		AppName:             "Commu Bin",
	})
}

func newTestWebHandlerWithBoardUseCase(session *testSessionUseCase, boardUseCase interface {
	GetBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error)
	GetAllBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error)
	CreateBoard(ctx context.Context, userID int64, name, description string) (string, error)
	UpdateBoard(ctx context.Context, boardUUID string, userID int64, name, description string) error
	DeleteBoard(ctx context.Context, boardUUID string, userID int64) error
	SetBoardVisibility(ctx context.Context, boardUUID string, userID int64, hidden bool) error
}) *Handler {
	if session == nil {
		session = &testSessionUseCase{}
	}
	if boardUseCase == nil {
		boardUseCase = &testBoardUseCase{}
	}
	h, err := NewHandler(Dependencies{
		AccountUseCase:      &testAccountUseCase{},
		SessionUseCase:      session,
		UserUseCase:         &testUserUseCase{},
		BoardUseCase:        boardUseCase,
		PostUseCase:         &testPostUseCase{},
		CommentUseCase:      &testCommentUseCase{},
		NotificationUseCase: &testNotificationUseCase{},
		ReportUseCase:       &testReportUseCase{},
		OutboxAdminUseCase:  &testOutboxAdminUseCase{},
		ReactionUseCase:     &testReactionUseCase{},
		AppName:             "Commu Bin",
	})
	if err != nil {
		panic(err)
	}
	return h
}

func newTestWebEngine() *gin.Engine {
	return newTestWebEngineWithSession(&testSessionUseCase{})
}

func newTestWebEngineWithSession(session *testSessionUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	newTestWebHandlerWithSession(session).RegisterRoutes(r)
	return r
}

func newTestWebEngineWithBoardUseCase(session *testSessionUseCase, boardUseCase interface {
	GetBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error)
	GetAllBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error)
	CreateBoard(ctx context.Context, userID int64, name, description string) (string, error)
	UpdateBoard(ctx context.Context, boardUUID string, userID int64, name, description string) error
	DeleteBoard(ctx context.Context, boardUUID string, userID int64) error
	SetBoardVisibility(ctx context.Context, boardUUID string, userID int64, hidden bool) error
}) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	newTestWebHandlerWithBoardUseCase(session, boardUseCase).RegisterRoutes(r)
	return r
}

func newTestWebEngineWithDependencies(session *testSessionUseCase, boardUseCase *testBoardUseCase, postUseCase *testPostUseCase, commentUseCase *testCommentUseCase, notificationUseCase *testNotificationUseCase, reportUseCase *testReportUseCase, outboxUseCase *testOutboxAdminUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h, err := newTestHandlerWithDependencies(session, boardUseCase, postUseCase, commentUseCase, notificationUseCase, reportUseCase, outboxUseCase)
	if err != nil {
		panic(err)
	}
	h.RegisterRoutes(r)
	return r
}

func authRequest(method, path string) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "admin-token"})
	return req, httptest.NewRecorder()
}

func TestHandler_RenderCoreScreens(t *testing.T) {
	r := newTestWebEngine()

	t.Run("home", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Feed")
		assert.Contains(t, rr.Body.String(), "General")
		assert.Contains(t, rr.Body.String(), "bottom-nav")
		assert.Contains(t, rr.Body.String(), "overlay-panel")
		assert.Contains(t, rr.Body.String(), "Hot")
	})

	t.Run("login", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/login?redirect=/me", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Login")
		assert.Contains(t, rr.Body.String(), "Access desk")
		assert.Contains(t, rr.Body.String(), "Create account")
		assert.Contains(t, rr.Body.String(), "Continue as guest")
	})

	t.Run("profile", func(t *testing.T) {
		req, rr := authRequest(http.MethodGet, "/me")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "My page")
		assert.Contains(t, rr.Body.String(), "alice@example.com")
		assert.Contains(t, rr.Body.String(), "Account overview")
		assert.Contains(t, rr.Body.String(), "Email not verified")
		assert.Contains(t, rr.Body.String(), "Draft title")
	})

	t.Run("compose", func(t *testing.T) {
		req, rr := authRequest(http.MethodGet, "/boards/general-uuid/posts/new")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "New post")
		assert.Contains(t, rr.Body.String(), "Writing room")
		assert.Contains(t, rr.Body.String(), "Publishing checks")
		assert.Contains(t, rr.Body.String(), "Public")
	})

	t.Run("post detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/posts/post-uuid", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Feed title")
		assert.Contains(t, rr.Body.String(), "Lead summary sentence.")
		assert.NotContains(t, rr.Body.String(), "At a glance")
		assert.NotContains(t, rr.Body.String(), "Quick jumps")
		assert.Contains(t, rr.Body.String(), "Comments")
		assert.Contains(t, rr.Body.String(), "Reading map")
		assert.Contains(t, rr.Body.String(), "Quick actions")
		assert.Contains(t, rr.Body.String(), "Login to react")
		assert.Contains(t, rr.Body.String(), "Sign in to reply")
		assert.Contains(t, rr.Body.String(), "post-meta")
		assert.NotContains(t, rr.Body.String(), "post-summary")
		assert.Contains(t, rr.Body.String(), "report-toggle")
		assert.Contains(t, rr.Body.String(), "Report")
		assert.Contains(t, rr.Body.String(), "Attachments")
		assert.Contains(t, rr.Body.String(), `<a href="https://example.com"`)
		assert.Contains(t, rr.Body.String(), `>docs</a>`)
		assert.Contains(t, rr.Body.String(), `/api/v1/posts/post-uuid/attachments/attach-uuid/file`)
		assert.Contains(t, rr.Body.String(), `/api/v1/posts/post-uuid/attachments/attach-uuid/preview`)
		assert.NotContains(t, rr.Body.String(), "<pre class=\"markdown\">")
		assert.NotContains(t, rr.Body.String(), "<script>alert(1)</script>")
		assert.Contains(t, rr.Body.String(), "Looks good.")
		assert.Contains(t, rr.Body.String(), `/reports`)
		assert.NotContains(t, rr.Body.String(), `/comments/comment-uuid/reactions`)
		assert.NotContains(t, rr.Body.String(), `target_type" value="comment"`)
	})

	t.Run("guest navigation hides auth-only links", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		body := rr.Body.String()
		assert.NotContains(t, body, `href="/me"`)
		assert.NotContains(t, body, `href="/notifications"`)
		assert.NotContains(t, body, `href="/logout"`)
		assert.NotContains(t, body, `sidebar-kicker">Account`)
		assert.Contains(t, body, `sidebar-dir-title">Community map`)
		assert.Equal(t, 1, strings.Count(body, `href="/signup"`))
		assert.Contains(t, body, `href="/login"`)
		assert.Contains(t, body, `bottom-nav-item`)
		assert.Contains(t, body, `>Menu</span>`)
	})

	t.Run("draft edit", func(t *testing.T) {
		req, rr := authRequest(http.MethodGet, "/posts/draft-uuid/edit")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Edit draft")
		assert.Contains(t, rr.Body.String(), "Draft title")
		assert.Contains(t, rr.Body.String(), "Publish")
	})

	t.Run("admin boards", func(t *testing.T) {
		req, rr := authRequest(http.MethodGet, "/admin/boards")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Boards")
		assert.Contains(t, rr.Body.String(), "Board directory")
		assert.Contains(t, rr.Body.String(), "Hidden")
		assert.Contains(t, rr.Body.String(), "Visible")
		assert.Contains(t, rr.Body.String(), "Show")
		assert.Contains(t, rr.Body.String(), "Visible")
		assert.Contains(t, rr.Body.String(), "Total")
	})

	t.Run("admin reports", func(t *testing.T) {
		req, rr := authRequest(http.MethodGet, "/admin/reports")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Moderation queue")
		assert.Contains(t, rr.Body.String(), "Moderation desk")
		assert.Contains(t, rr.Body.String(), "Report")
		assert.Contains(t, rr.Body.String(), "Resolve")
		assert.Contains(t, rr.Body.String(), "accepted / rejected")
	})

	t.Run("admin outbox", func(t *testing.T) {
		req, rr := authRequest(http.MethodGet, "/admin/outbox")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Dead outbox")
		assert.Contains(t, rr.Body.String(), "Delivery queue")
		assert.Contains(t, rr.Body.String(), "Attempts")
		assert.Contains(t, rr.Body.String(), "Requeue")
		assert.Contains(t, rr.Body.String(), "Discard")
	})

	t.Run("admin snapshot", func(t *testing.T) {
		req, rr := authRequest(http.MethodGet, "/admin")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Admin Snapshot")
		assert.Contains(t, rr.Body.String(), "Operations snapshot")
		assert.Contains(t, rr.Body.String(), "Reports")
		assert.Contains(t, rr.Body.String(), "Boards")
	})
}

func TestHandler_RenderPaginationLinks(t *testing.T) {
	feedCursor := "feed-next"
	boardCursor := "board-next"
	tagCursor := "tag-next"
	searchCursor := "search-next"
	draftCursor := "draft-next"
	notificationCursor := "notification-next"
	boardListCursor := "boards-next"
	commentCursor := "comment-next"
	reportLastID := int64(9)
	outboxLastID := "dead-next"

	postUseCase := &testPostUseCase{
		feedList:   &model.PostList{Posts: []model.Post{{UUID: "feed-post", Title: "Feed post", Content: "Body", BoardUUID: "general-uuid", AuthorUUID: "user-uuid"}}, Limit: 20, HasMore: true, NextCursor: &feedCursor},
		boardList:  &model.PostList{Posts: []model.Post{{UUID: "board-post", Title: "Board post", Content: "Body", BoardUUID: "general-uuid", AuthorUUID: "user-uuid"}}, Limit: 20, HasMore: true, NextCursor: &boardCursor},
		tagList:    &model.PostList{Posts: []model.Post{{UUID: "tag-post", Title: "Tag post", Content: "Body", BoardUUID: "general-uuid", AuthorUUID: "user-uuid"}}, Limit: 20, HasMore: true, NextCursor: &tagCursor},
		searchList: &model.PostList{Posts: []model.Post{{UUID: "search-post", Title: "Search post", Content: "Body", BoardUUID: "general-uuid", AuthorUUID: "user-uuid"}}, Limit: 20, HasMore: true, NextCursor: &searchCursor},
		draftList:  &model.PostList{Posts: []model.Post{{UUID: "draft-uuid-2", Title: "Draft next", Content: "Draft body", BoardUUID: "general-uuid", AuthorUUID: "user-uuid"}}, Limit: 20, HasMore: true, NextCursor: &draftCursor},
	}
	commentUseCase := &testCommentUseCase{
		comments: &model.CommentList{
			Comments: []model.Comment{{UUID: "comment-1", Content: "First", AuthorUUID: "user-uuid", PostUUID: "post-uuid"}, {UUID: "comment-2", Content: "Second", AuthorUUID: "user-uuid", PostUUID: "post-uuid"}},
			Limit:    2,
			HasMore:  true,
			NextCursor: func() *string {
				return &commentCursor
			}(),
		},
	}
	notificationUseCase := &testNotificationUseCase{
		notifications: &model.NotificationList{Notifications: []model.Notification{{UUID: "notif-1", MessageKey: "post_commented", CreatedAt: time.Unix(60, 0)}}, Limit: 20, HasMore: true, NextCursor: &notificationCursor},
	}
	reportUseCase := &testReportUseCase{
		reports: &model.ReportList{Reports: []model.Report{{ID: 11, ReasonCode: "spam", TargetType: "post", TargetUUID: "post-uuid", ReporterUUID: "reporter-uuid", Status: "pending", CreatedAt: time.Unix(70, 0), UpdatedAt: time.Unix(70, 0)}}, Limit: 20, HasMore: true, NextLastID: &reportLastID},
	}
	outboxUseCase := &testOutboxAdminUseCase{
		outbox: &model.OutboxDeadMessageList{Messages: []model.OutboxDeadMessage{{ID: "dead-1", EventName: "post.changed", AttemptCount: 1, LastError: "boom", OccurredAt: time.Unix(80, 0), NextAttemptAt: time.Unix(90, 0)}}, Limit: 20, HasMore: true, NextLastID: &outboxLastID},
	}
	boardUseCase := &testBoardUseCase{
		boards:    &model.BoardList{Boards: []model.Board{{UUID: "general-uuid", Name: "General", Description: "Visible board"}}, Limit: 20, HasMore: true, NextCursor: &boardListCursor},
		allBoards: &model.BoardList{Boards: []model.Board{{UUID: "general-uuid", Name: "General", Description: "Visible board"}}, Limit: 20, HasMore: true, NextCursor: &boardListCursor},
	}
	r := newTestWebEngineWithDependencies(&testSessionUseCase{}, boardUseCase, postUseCase, commentUseCase, notificationUseCase, reportUseCase, outboxUseCase)

	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "feed", path: "/", want: "page=2"},
		{name: "board", path: "/boards/general-uuid", want: "page=2"},
		{name: "tag", path: "/tags/playwright", want: "page=2"},
		{name: "search", path: "/search?q=go", want: "page=2"},
		{name: "drafts", path: "/me", want: "page=2"},
		{name: "notifications", path: "/notifications", want: "page=2"},
		{name: "admin reports", path: "/admin/reports", want: "page=2"},
		{name: "admin outbox", path: "/admin/outbox", want: "page=2"},
		{name: "admin boards", path: "/admin/boards", want: "page=2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			if strings.HasPrefix(tc.path, "/me") || strings.HasPrefix(tc.path, "/notifications") || strings.HasPrefix(tc.path, "/admin") || strings.HasPrefix(tc.path, "/posts/") {
				req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "admin-token"})
			}
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			require.Equal(t, http.StatusOK, rr.Code)
			body := rr.Body.String()
			assert.Contains(t, body, "Prev")
			assert.Contains(t, body, "Next")
			assert.Contains(t, body, tc.want)
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/posts/post-uuid", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "admin-token"})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "Load more comments")
	assert.Contains(t, body, "comment_cursor=comment-next")
}

func TestHandler_RenderCoreScreens_DisablesNewPostWhenNoBoards(t *testing.T) {
	r := newTestWebEngineWithBoardUseCase(&testSessionUseCase{}, &testBoardUseCaseEmpty{})

	req, rr := authRequest(http.MethodGet, "/")
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "Start thread")
	assert.Contains(t, body, "bottom-nav-item is-disabled")
	assert.Contains(t, body, "aria-disabled=\"true\"")
	assert.NotContains(t, body, "href=\"/login\"")
}

func TestHandler_RenderEmptyStateSurfaces(t *testing.T) {
	t.Run("feed", func(t *testing.T) {
		postUseCase := &testPostUseCase{feedList: &model.PostList{}}
		r := newTestWebEngineWithDependencies(&testSessionUseCase{}, &testBoardUseCase{}, postUseCase, &testCommentUseCase{}, &testNotificationUseCase{}, &testReportUseCase{}, &testOutboxAdminUseCase{})

		req, rr := authRequest(http.MethodGet, "/")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		body := rr.Body.String()
		assert.Contains(t, body, "No conversations yet")
		assert.Contains(t, body, "Start the first thread")
		assert.Contains(t, body, "Write the first post")
		assert.Contains(t, body, "empty-state-icon")
		assert.Contains(t, body, "forum")
	})

	t.Run("search", func(t *testing.T) {
		postUseCase := &testPostUseCase{searchList: &model.PostList{}}
		r := newTestWebEngineWithDependencies(&testSessionUseCase{}, &testBoardUseCase{}, postUseCase, &testCommentUseCase{}, &testNotificationUseCase{}, &testReportUseCase{}, &testOutboxAdminUseCase{})

		req := httptest.NewRequest(http.MethodGet, "/search?q=void", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		body := rr.Body.String()
		assert.Contains(t, body, "Nothing matched")
		assert.Contains(t, body, "Try a broader term or browse the full feed")
		assert.Contains(t, body, "Browse all posts")
		assert.Contains(t, body, "search_off")
	})

	t.Run("drafts", func(t *testing.T) {
		postUseCase := &testPostUseCase{draftList: &model.PostList{}}
		r := newTestWebEngineWithDependencies(&testSessionUseCase{}, &testBoardUseCase{}, postUseCase, &testCommentUseCase{}, &testNotificationUseCase{}, &testReportUseCase{}, &testOutboxAdminUseCase{})

		req, rr := authRequest(http.MethodGet, "/me")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		body := rr.Body.String()
		assert.Contains(t, body, "No drafts yet")
		assert.Contains(t, body, "Save a draft to come back to it later")
		assert.Contains(t, body, "draft")
	})

	t.Run("boards", func(t *testing.T) {
		r := newTestWebEngineWithBoardUseCase(&testSessionUseCase{}, &testBoardUseCaseEmpty{})

		req, rr := authRequest(http.MethodGet, "/admin/boards")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		body := rr.Body.String()
		assert.Contains(t, body, "No boards yet")
		assert.Contains(t, body, "Create the first board to shape the directory")
		assert.Contains(t, body, "dashboard")
	})

	t.Run("comments", func(t *testing.T) {
		commentUseCase := &testCommentUseCase{comments: &model.CommentList{}}
		r := newTestWebEngineWithDependencies(&testSessionUseCase{}, &testBoardUseCase{}, &testPostUseCase{}, commentUseCase, &testNotificationUseCase{}, &testReportUseCase{}, &testOutboxAdminUseCase{})

		req, rr := authRequest(http.MethodGet, "/posts/post-uuid")
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusOK, rr.Code)
		body := rr.Body.String()
		assert.Contains(t, body, "No replies yet")
		assert.Contains(t, body, "Use the reply form above to open the discussion.")
		assert.Contains(t, body, "comment")
	})
}

func TestHandler_RenderCoreScreens_AutoIssuesGuestSessionOnPublicPages(t *testing.T) {
	session := &testSessionUseCase{}
	r := newTestWebEngineWithSession(session)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.GreaterOrEqual(t, session.issueGuestCalls, 1)
	cookies := strings.Join(rr.Header()["Set-Cookie"], "\n")
	assert.Contains(t, cookies, sessionCookieName+"=guest-token")
	assert.Contains(t, cookies, "Max-Age=2592000")
}

func TestHandler_RenderCoreScreens_PreservesSessionCookieOnTransientValidationFailure(t *testing.T) {
	session := &testSessionUseCase{validateErr: customerror.WrapRepository("lookup session", errors.New("db down"))}
	r := newTestWebEngineWithSession(session)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Feed")
	assert.GreaterOrEqual(t, session.validateCalls, 1)
	for _, header := range rr.Header()["Set-Cookie"] {
		assert.NotContains(t, header, sessionCookieName+"=")
	}
}

func TestHandler_RenderCoreScreens_ClearsInvalidCookieWhenTokenInvalid(t *testing.T) {
	session := &testSessionUseCase{validateErr: customerror.ErrInvalidToken}
	r := newTestWebEngineWithSession(session)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Feed")
	assert.GreaterOrEqual(t, session.validateCalls, 1)
	assert.Contains(t, strings.Join(rr.Header()["Set-Cookie"], "\n"), sessionCookieName+"=;")
}

func TestHandler_LoginSubmit_RejectsMissingOrMismatchedCSRF(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		session := &testSessionUseCase{}
		r := newTestWebEngineWithSession(session)

		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=alice&password=pw"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusForbidden, rr.Code)
		assert.Equal(t, 0, session.loginCalls)
		assert.NotContains(t, strings.Join(rr.Header()["Set-Cookie"], "\n"), sessionCookieName+"=")
	})

	t.Run("mismatched", func(t *testing.T) {
		session := &testSessionUseCase{}
		r := newTestWebEngineWithSession(session)

		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=alice&password=pw"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set(csrfHeaderName, "submitted")
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "cookie"})
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		require.Equal(t, http.StatusForbidden, rr.Code)
		assert.Equal(t, 0, session.loginCalls)
		assert.NotContains(t, strings.Join(rr.Header()["Set-Cookie"], "\n"), sessionCookieName+"=")
	})
}

func TestHandler_LoginSubmit_SetsSecureSessionCookieWhenRequestIsSecure(t *testing.T) {
	session := &testSessionUseCase{}
	r := newTestWebEngineWithSession(session)

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=alice&password=pw"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(csrfHeaderName, "csrf-token")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, 1, session.loginCalls)
	cookies := strings.Join(rr.Header()["Set-Cookie"], "\n")
	assert.Contains(t, cookies, sessionCookieName+"=token")
	assert.Contains(t, cookies, "HttpOnly")
	assert.Contains(t, cookies, "SameSite=Lax")
	assert.Contains(t, cookies, "Secure")
}

func TestHandler_GuestLoginSubmit_SetsPersistentSessionCookie(t *testing.T) {
	session := &testSessionUseCase{}
	r := newTestWebEngineWithSession(session)

	req := httptest.NewRequest(http.MethodPost, "/guest", strings.NewReader("redirect=/me"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(csrfHeaderName, "csrf-token")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, 1, session.issueGuestCalls)
	cookies := strings.Join(rr.Header()["Set-Cookie"], "\n")
	assert.Contains(t, cookies, sessionCookieName+"=guest-token")
	assert.Contains(t, cookies, "Max-Age=2592000")
	assert.Contains(t, cookies, "Expires=")
	assert.Contains(t, rr.Header().Get("Location"), "/me")
}

func TestHandler_LogoutSubmit_ReturnsErrorAndKeepsCookieWhenLogoutFails(t *testing.T) {
	session := &testSessionUseCase{logoutErr: errors.New("db down")}
	r := newTestWebEngineWithSession(session)

	req := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(csrfHeaderName, "csrf-token")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, 1, session.logoutCalls)
	assert.NotContains(t, strings.Join(rr.Header()["Set-Cookie"], "\n"), sessionCookieName+"=;")
}

func TestHandler_LogoutSubmit_ClearsCookieOnSuccess(t *testing.T) {
	session := &testSessionUseCase{}
	r := newTestWebEngineWithSession(session)

	req := httptest.NewRequest(http.MethodPost, "/logout", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(csrfHeaderName, "csrf-token")
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusSeeOther, rr.Code)
	assert.Equal(t, 1, session.logoutCalls)
	cookies := strings.Join(rr.Header()["Set-Cookie"], "\n")
	assert.Contains(t, cookies, sessionCookieName+"=;")
	assert.Contains(t, cookies, "Expires=Thu, 01 Jan 1970 00:00:00 GMT")
}

func TestSafeRedirectTarget(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: "/"},
		{name: "relative", raw: "/me", want: "/me"},
		{name: "absolute", raw: "https://evil.example", want: "/"},
		{name: "protocol-relative", raw: "//evil.example", want: "/"},
		{name: "malformed", raw: "://bad", want: "/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, safeRedirectTarget(tc.raw))
		})
	}
}
