package web

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
)

type Dependencies struct {
	AccountUseCase interface {
		DeleteMyAccount(ctx context.Context, userID int64, password string) error
		UpgradeGuestAccount(ctx context.Context, userID int64, currentToken, username, email, password string) (string, error)
		RequestEmailVerification(ctx context.Context, userID int64) error
		ConfirmEmailVerification(ctx context.Context, token string) error
		RequestPasswordReset(ctx context.Context, email string) error
		ConfirmPasswordReset(ctx context.Context, token, newPassword string) error
	}
	SessionUseCase interface {
		ValidateTokenToId(ctx context.Context, token string) (int64, error)
		Login(ctx context.Context, username, password string) (string, error)
		IssueGuestToken(ctx context.Context) (string, error)
		RotateToken(ctx context.Context, userID int64, currentToken string) (string, error)
		Logout(ctx context.Context, token string) error
		InvalidateUserSessions(ctx context.Context, userID int64) error
	}
	UserUseCase interface {
		GetMe(ctx context.Context, userID int64) (*model.User, error)
		DeleteMe(ctx context.Context, userID int64, password string) error
		SignUp(ctx context.Context, username, email, password string) (string, error)
		IssueGuestAccount(ctx context.Context) (int64, error)
		UpgradeGuest(ctx context.Context, userID int64, username, email, password string) error
		GetUserSuspension(ctx context.Context, adminID int64, targetUserUUID string) (*model.UserSuspension, error)
		SuspendUser(ctx context.Context, adminID int64, targetUserUUID, reason string, duration model.SuspensionDuration) error
		UnsuspendUser(ctx context.Context, adminID int64, targetUserUUID string) error
	}
	BoardUseCase interface {
		GetBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error)
		GetAllBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error)
		CreateBoard(ctx context.Context, userID int64, name, description string) (string, error)
		UpdateBoard(ctx context.Context, boardUUID string, userID int64, name, description string) error
		DeleteBoard(ctx context.Context, boardUUID string, userID int64) error
		SetBoardVisibility(ctx context.Context, boardUUID string, userID int64, hidden bool) error
	}
	ReactionUseCase interface {
		SetReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType, reactionType model.ReactionType) (bool, error)
		DeleteReaction(ctx context.Context, userID int64, targetUUID string, targetType model.ReactionTargetType) error
	}
	PostUseCase interface {
		CreatePost(ctx context.Context, title, content string, tags []string, mentionedUsernames []string, authorID int64, boardUUID string) (string, error)
		CreateDraftPost(ctx context.Context, title, content string, tags []string, mentionedUsernames []string, authorID int64, boardUUID string) (string, error)
		GetPostsList(ctx context.Context, boardUUID string, sort string, window string, limit int, cursor string) (*model.PostList, error)
		GetMyDraftPosts(ctx context.Context, authorID int64, limit int, cursor string) (*model.PostList, error)
		GetFeed(ctx context.Context, sort string, window string, limit int, cursor string) (*model.PostList, error)
		SearchPosts(ctx context.Context, query string, sort string, window string, limit int, cursor string) (*model.PostList, error)
		GetPostsByTag(ctx context.Context, tagName string, sort string, window string, limit int, cursor string) (*model.PostList, error)
		GetPostDetail(ctx context.Context, postUUID string) (*model.PostDetail, error)
		GetDraftPost(ctx context.Context, postUUID string, userID int64) (*model.PostDetail, error)
		PublishPost(ctx context.Context, postUUID string, authorID int64) error
		UpdatePost(ctx context.Context, postUUID string, authorID int64, title, content string, tags []string) error
		DeletePost(ctx context.Context, postUUID string, authorID int64) error
	}
	CommentUseCase interface {
		CreateComment(ctx context.Context, content string, mentionedUsernames []string, authorID int64, postUUID string, parentUUID *string) (string, error)
		GetCommentsByPost(ctx context.Context, postUUID string, limit int, cursor string) (*model.CommentList, error)
		UpdateComment(ctx context.Context, commentUUID string, authorID int64, content string) error
		DeleteComment(ctx context.Context, commentUUID string, authorID int64) error
	}
	NotificationUseCase interface {
		GetMyNotifications(ctx context.Context, userID int64, limit int, cursor string) (*model.NotificationList, error)
		GetMyUnreadNotificationCount(ctx context.Context, userID int64) (int, error)
		MarkMyNotificationRead(ctx context.Context, userID int64, notificationUUID string) error
		MarkAllMyNotificationsRead(ctx context.Context, userID int64) error
	}
	ReportUseCase interface {
		CreateReport(ctx context.Context, reporterUserID int64, targetType model.ReportTargetType, targetUUID string, reasonCode model.ReportReasonCode, reasonDetail string) (int64, error)
		GetReports(ctx context.Context, adminID int64, status *model.ReportStatus, limit int, lastID int64) (*model.ReportList, error)
		ResolveReport(ctx context.Context, adminID, reportID int64, status model.ReportStatus, resolutionNote string) error
	}
	OutboxAdminUseCase interface {
		GetDeadMessages(ctx context.Context, adminID int64, limit int, lastID string) (*model.OutboxDeadMessageList, error)
		RequeueDeadMessage(ctx context.Context, adminID int64, messageID string) error
		DiscardDeadMessage(ctx context.Context, adminID int64, messageID string) error
	}
	Logger interface {
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
		Error(msg string, args ...any)
	}
	AppName string
}

type ShellData struct {
	AppName         string
	Title           string
	Description     string
	ActiveNav       string
	ComposeURL      string
	CurrentUser     *model.User
	IsAuthenticated bool
	IsAdmin         bool
	UnreadCount     int
	CSRFToken       string
	Redirect        string
	Boards          []model.Board
	BoardMap        map[string]model.Board
}

type PageData struct {
	Shell             ShellData
	Kind              string
	Message           string
	Query             string
	SortValue         string
	BoardUUID         string
	TagName           string
	PostUUID          string
	EditMode          string
	Redirect          string
	Feed              *model.PostList
	PostDetail        *model.PostDetail
	Drafts            *model.PostList
	Notifications     *model.NotificationList
	Reports           *model.ReportList
	Outbox            *model.OutboxDeadMessageList
	AdminBoards       *model.BoardList
	AdminBoardTarget  *model.Board
	BoardVisibleCount int
	BoardHiddenCount  int
	Suspension        *model.UserSuspension
	TitleInput        string
	ContentInput      string
	TagsInput         string
	ReasonCode        string
	ReasonDetail      string
	ResolutionNote    string
	StatusValue       string
	LoginUsername     string
	LoginEmail        string
	VerifyToken       string
	ResetToken        string
	ErrorMessage      string
}
