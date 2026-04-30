package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	postsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/post"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

const (
	sessionCookieName = "session"
	csrfCookieName    = "csrf_token"
	csrfHeaderName    = "X-CSRF-Token"
	webDefaultLimit   = 20
	guestSessionAge   = 30 * 24 * time.Hour
)

var (
	markdownRenderer = goldmark.New(
		goldmark.WithExtensions(extension.GFM),
	)
	markdownPolicy = bluemonday.UGCPolicy()
)

type Handler struct {
	deps      Dependencies
	templates *template.Template
	assetsFS  fs.FS
}

func NewHandler(deps Dependencies) (*Handler, error) {
	funcMap := template.FuncMap{
		"dict": func(values ...any) map[string]any {
			m := make(map[string]any, len(values)/2)
			for i := 0; i+1 < len(values); i += 2 {
				key, _ := values[i].(string)
				m[key] = values[i+1]
			}
			return m
		},
		"queryEscape": url.QueryEscape,
		"shortUUID": func(value string) string {
			value = strings.TrimSpace(value)
			if len(value) <= 8 {
				return value
			}
			return value[:8]
		},
		"truncate": func(value string, max int) string {
			value = strings.TrimSpace(value)
			if max <= 0 || len(value) <= max {
				return value
			}
			if max < 4 {
				return value[:max]
			}
			return value[:max-1] + "…"
		},
		"normalizeContent": func(value string) string {
			return normalizeContent(value)
		},
		"renderMarkdown": func(value string) template.HTML {
			return renderMarkdown(value)
		},
		"pageURL": func(base string, params ...any) string {
			return pageURL(base, params...)
		},
		"listPageURL": func(data any, page int) string {
			return listPageURL(data, page)
		},
		"cursorValue": func(value *string) string {
			if value == nil {
				return ""
			}
			return strings.TrimSpace(*value)
		},
		"int64Value": func(value *int64) string {
			if value == nil {
				return ""
			}
			return strconv.FormatInt(*value, 10)
		},
		"formatTime": func(t time.Time) string {
			if t.IsZero() {
				return "-"
			}
			return t.In(time.Local).Format("2006-01-02 15:04")
		},
		"boardName": func(boardMap map[string]model.Board, uuid string) string {
			if boardMap == nil {
				return uuid
			}
			if board, ok := boardMap[uuid]; ok && board.Name != "" {
				return board.Name
			}
			return uuid
		},
		"reactionActive": func(current *entity.ReactionType, expected string) bool {
			return current != nil && string(*current) == expected
		},
	}
	templates, err := template.New("layout").Funcs(funcMap).ParseFS(embeddedAssets, "templates/*.tmpl")
	if err != nil {
		return nil, err
	}
	assets, err := fs.Sub(embeddedAssets, "static")
	if err != nil {
		return nil, err
	}
	return &Handler{deps: deps, templates: templates, assetsFS: assets}, nil
}

func normalizeContent(value string) string {
	if value == "" {
		return value
	}
	return strings.NewReplacer(
		`\r\n`, "\n",
		`\n`, "\n",
		`\r`, "\n",
		`\t`, "\t",
	).Replace(value)
}

func renderMarkdown(value string) template.HTML {
	value = normalizeContent(value)
	if value == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := markdownRenderer.Convert([]byte(value), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(value))
	}
	return template.HTML(markdownPolicy.SanitizeBytes(buf.Bytes()))
}

func pageURL(base string, params ...any) string {
	if strings.TrimSpace(base) == "" {
		base = "/"
	}
	if len(params)%2 != 0 {
		return base
	}
	values := url.Values{}
	for i := 0; i < len(params); i += 2 {
		key, _ := params[i].(string)
		if key == "" {
			continue
		}
		switch value := params[i+1].(type) {
		case string:
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			values.Set(key, value)
		case *string:
			if value == nil {
				continue
			}
			trimmed := strings.TrimSpace(*value)
			if trimmed == "" {
				continue
			}
			values.Set(key, trimmed)
		case int:
			if value == 0 {
				continue
			}
			values.Set(key, strconv.Itoa(value))
		case *int64:
			if value == nil {
				continue
			}
			values.Set(key, strconv.FormatInt(*value, 10))
		case int64:
			if value == 0 {
				continue
			}
			values.Set(key, strconv.FormatInt(value, 10))
		default:
			if value == nil {
				continue
			}
			str := strings.TrimSpace(fmt.Sprint(value))
			if str == "" {
				continue
			}
			values.Set(key, str)
		}
	}
	if len(values) == 0 {
		return base
	}
	return base + "?" + values.Encode()
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.GET("/assets/*filepath", h.serveAsset)

	r.GET("/", h.handleFeed)
	r.GET("/boards/:boardUUID", h.handleBoardFeed)
	r.GET("/boards/:boardUUID/posts/new", h.handleNewPost)
	r.GET("/boards/:boardUUID/posts/drafts/new", h.handleNewDraft)
	r.POST("/boards/:boardUUID/posts", h.requireHTMLAuth(h.handleCreatePostSubmit))
	r.POST("/boards/:boardUUID/posts/drafts", h.requireHTMLAuth(h.handleCreateDraftSubmit))
	r.GET("/posts/:postUUID", h.handlePostDetail)
	r.GET("/posts/:postUUID/edit", h.handleEditDraft)
	r.POST("/posts/:postUUID", h.requireHTMLAuth(h.handleUpdatePostSubmit))
	r.POST("/posts/:postUUID/publish", h.requireHTMLAuth(h.handlePublishPostSubmit))
	r.POST("/posts/:postUUID/delete", h.requireHTMLAuth(h.handleDeletePostSubmit))
	r.POST("/posts/:postUUID/comments", h.requireHTMLAuth(h.handleCreateCommentSubmit))
	r.POST("/posts/:postUUID/reactions", h.requireHTMLAuth(h.handlePostReactionSubmit))
	r.POST("/comments/:commentUUID/reactions", h.requireHTMLAuth(h.handleCommentReactionSubmit))
	r.POST("/comments/:commentUUID", h.requireHTMLAuth(h.handleUpdateCommentSubmit))
	r.POST("/comments/:commentUUID/delete", h.requireHTMLAuth(h.handleDeleteCommentSubmit))
	r.POST("/reports", h.requireHTMLAuth(h.handleCreateReportSubmit))
	r.GET("/tags/:tagName", h.handleTagFeed)
	r.GET("/search", h.handleSearch)

	r.GET("/signup", h.handleSignupPage)
	r.POST("/signup", h.handleSignupSubmit)
	r.GET("/login", h.handleLoginPage)
	r.POST("/login", h.handleLoginSubmit)
	r.POST("/guest", h.handleGuestLoginSubmit)
	r.POST("/logout", h.handleLogoutSubmit)
	r.GET("/verify-email", h.handleVerifyEmailPage)
	r.POST("/verify-email", h.handleVerifyEmailSubmit)
	r.GET("/reset-password", h.handleResetPasswordPage)
	r.POST("/reset-password", h.handleResetPasswordSubmit)

	r.GET("/me", h.requireHTMLAuth(h.handleMePage))
	r.POST("/me/delete", h.requireHTMLAuth(h.handleMeDeleteSubmit))
	r.POST("/me/verify-email", h.requireHTMLAuth(h.handleMeRequestVerifyEmailSubmit))
	r.GET("/me/upgrade", h.requireHTMLAuth(h.handleMeUpgradePage))
	r.POST("/me/upgrade", h.requireHTMLAuth(h.handleMeUpgradeSubmit))
	r.GET("/notifications", h.requireHTMLAuth(h.handleNotificationsPage))
	r.POST("/notifications/read-all", h.requireHTMLAuth(h.handleNotificationsReadAllSubmit))
	r.POST("/notifications/:notificationUUID/read", h.requireHTMLAuth(h.handleNotificationReadSubmit))

	r.GET("/admin", h.requireHTMLAdmin(h.handleAdminDashboard))
	r.GET("/admin/reports", h.requireHTMLAdmin(h.handleAdminReportsPage))
	r.POST("/admin/reports/:reportID/resolve", h.requireHTMLAdmin(h.handleAdminReportResolveSubmit))
	r.GET("/admin/outbox", h.requireHTMLAdmin(h.handleAdminOutboxPage))
	r.POST("/admin/outbox/dead/:messageID/requeue", h.requireHTMLAdmin(h.handleAdminOutboxRequeueSubmit))
	r.POST("/admin/outbox/dead/:messageID/discard", h.requireHTMLAdmin(h.handleAdminOutboxDiscardSubmit))
	r.GET("/admin/boards", h.requireHTMLAdmin(h.handleAdminBoardsPage))
	r.POST("/admin/boards", h.requireHTMLAdmin(h.handleAdminBoardCreateSubmit))
	r.GET("/admin/boards/:boardUUID/edit", h.requireHTMLAdmin(h.handleAdminBoardEditPage))
	r.POST("/admin/boards/:boardUUID", h.requireHTMLAdmin(h.handleAdminBoardUpdateSubmit))
	r.POST("/admin/boards/:boardUUID/delete", h.requireHTMLAdmin(h.handleAdminBoardDeleteSubmit))
	r.POST("/admin/boards/:boardUUID/visibility", h.requireHTMLAdmin(h.handleAdminBoardVisibilitySubmit))
	r.GET("/admin/users/:userUUID/suspension", h.requireHTMLAdmin(h.handleAdminSuspensionPage))
	r.POST("/admin/users/:userUUID/suspension", h.requireHTMLAdmin(h.handleAdminSuspensionSubmit))
}

func (h *Handler) serveAsset(c *gin.Context) {
	name := strings.TrimPrefix(c.Param("filepath"), "/")
	if name == "" || strings.Contains(name, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	data, err := fs.ReadFile(h.assetsFS, name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	switch {
	case strings.HasSuffix(name, ".css"):
		c.Data(http.StatusOK, "text/css; charset=utf-8", data)
	case strings.HasSuffix(name, ".js"):
		c.Data(http.StatusOK, "text/javascript; charset=utf-8", data)
	case strings.HasSuffix(name, ".svg"):
		c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", data)
	default:
		c.Data(http.StatusOK, http.DetectContentType(data), data)
	}
}

func (h *Handler) handleFeed(c *gin.Context) {
	shell, ok := h.shellForRequest(c, "feed")
	if !ok {
		return
	}
	sortValue := strings.TrimSpace(c.Query("sort"))
	windowValue := strings.TrimSpace(c.Query("window"))
	limit := parsePageLimit(c.Query("limit"))
	page := parsePageNumber(c.Query("page"))
	feed, currentPage, hasMore, err := loadSequentialPage(c.Request.Context(), page, "", func(ctx context.Context, cursor string) (*model.PostList, string, bool, error) {
		list, err := h.deps.PostUseCase.GetFeed(ctx, sortValue, windowValue, limit, cursor)
		if err != nil {
			return nil, "", false, err
		}
		nextCursor := ""
		if list != nil && list.NextCursor != nil {
			nextCursor = strings.TrimSpace(*list.NextCursor)
		}
		return list, nextCursor, list != nil && list.HasMore && nextCursor != "", nil
	})
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "feed", ListBaseURL: "/", ListLimit: limit, Pagination: buildPaginationData(currentPage, hasMore), Feed: feed, SortValue: sortValue, WindowValue: windowValue})
}

func (h *Handler) handleBoardFeed(c *gin.Context) {
	boardUUID := strings.TrimSpace(c.Param("boardUUID"))
	shell, ok := h.shellForRequest(c, "boards")
	if !ok {
		return
	}
	sortValue := strings.TrimSpace(c.Query("sort"))
	windowValue := strings.TrimSpace(c.Query("window"))
	limit := parsePageLimit(c.Query("limit"))
	page := parsePageNumber(c.Query("page"))
	feed, currentPage, hasMore, err := loadSequentialPage(c.Request.Context(), page, "", func(ctx context.Context, cursor string) (*model.PostList, string, bool, error) {
		list, err := h.deps.PostUseCase.GetPostsList(ctx, boardUUID, sortValue, windowValue, limit, cursor)
		if err != nil {
			return nil, "", false, err
		}
		nextCursor := ""
		if list != nil && list.NextCursor != nil {
			nextCursor = strings.TrimSpace(*list.NextCursor)
		}
		return list, nextCursor, list != nil && list.HasMore && nextCursor != "", nil
	})
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "board", ListBaseURL: "/boards/" + boardUUID, ListLimit: limit, Pagination: buildPaginationData(currentPage, hasMore), BoardUUID: boardUUID, BoardName: h.lookupBoardName(c.Request.Context(), shell, boardUUID), Feed: feed, SortValue: sortValue, WindowValue: windowValue})
}

func (h *Handler) handleTagFeed(c *gin.Context) {
	tagName := strings.TrimSpace(c.Param("tagName"))
	shell, ok := h.shellForRequest(c, "tags")
	if !ok {
		return
	}
	sortValue := strings.TrimSpace(c.Query("sort"))
	windowValue := strings.TrimSpace(c.Query("window"))
	limit := parsePageLimit(c.Query("limit"))
	page := parsePageNumber(c.Query("page"))
	feed, currentPage, hasMore, err := loadSequentialPage(c.Request.Context(), page, "", func(ctx context.Context, cursor string) (*model.PostList, string, bool, error) {
		list, err := h.deps.PostUseCase.GetPostsByTag(ctx, tagName, sortValue, windowValue, limit, cursor)
		if err != nil {
			return nil, "", false, err
		}
		nextCursor := ""
		if list != nil && list.NextCursor != nil {
			nextCursor = strings.TrimSpace(*list.NextCursor)
		}
		return list, nextCursor, list != nil && list.HasMore && nextCursor != "", nil
	})
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "tag", ListBaseURL: "/tags/" + tagName, ListLimit: limit, Pagination: buildPaginationData(currentPage, hasMore), TagName: tagName, Feed: feed, SortValue: sortValue, WindowValue: windowValue})
}

func (h *Handler) handleSearch(c *gin.Context) {
	shell, ok := h.shellForRequest(c, "search")
	if !ok {
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	sortValue := strings.TrimSpace(c.Query("sort"))
	windowValue := strings.TrimSpace(c.Query("window"))
	limit := parsePageLimit(c.Query("limit"))
	page := parsePageNumber(c.Query("page"))
	var feed *model.PostList
	hasMore := false
	currentPage := 1
	var err error
	if query != "" {
		feed, currentPage, hasMore, err = loadSequentialPage(c.Request.Context(), page, "", func(ctx context.Context, cursor string) (*model.PostList, string, bool, error) {
			list, err := h.deps.PostUseCase.SearchPosts(ctx, query, sortValue, windowValue, limit, cursor)
			if err != nil {
				return nil, "", false, err
			}
			nextCursor := ""
			if list != nil && list.NextCursor != nil {
				nextCursor = strings.TrimSpace(*list.NextCursor)
			}
			return list, nextCursor, list != nil && list.HasMore && nextCursor != "", nil
		})
		if err != nil {
			h.renderUseCaseError(c, err)
			return
		}
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "search", ListBaseURL: "/search", ListLimit: limit, Pagination: buildPaginationData(currentPage, hasMore), Query: query, Feed: feed, SortValue: sortValue, WindowValue: windowValue})
}

func (h *Handler) handlePostDetail(c *gin.Context) {
	shell, ok := h.shellForRequest(c, "feed")
	if !ok {
		return
	}
	postUUID := strings.TrimSpace(c.Param("postUUID"))
	ctx := c.Request.Context()
	if shell.CurrentUser != nil {
		ctx = postsvc.WithViewerUserID(ctx, userIDFromModel(shell.CurrentUser))
	}
	detail, err := h.deps.PostUseCase.GetPostDetail(ctx, postUUID)
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	commentsLimit := parsePageLimit(c.Query("comment_limit"))
	commentsCursor := strings.TrimSpace(c.Query("comment_cursor"))
	comments, err := h.deps.CommentUseCase.GetCommentsByPost(ctx, postUUID, commentsLimit, commentsCursor)
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "post", ListBaseURL: "/posts/" + postUUID, PostDetail: detail, PostUUID: postUUID, Comments: comments})
}

func (h *Handler) handleNewPost(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "compose")
	if !ok {
		return
	}
	boardUUID := strings.TrimSpace(c.Param("boardUUID"))
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "compose", BoardUUID: boardUUID, BoardName: h.lookupBoardName(c.Request.Context(), shell, boardUUID), EditMode: "publish"})
}

func (h *Handler) handleNewDraft(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "compose")
	if !ok {
		return
	}
	boardUUID := strings.TrimSpace(c.Param("boardUUID"))
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "compose", BoardUUID: boardUUID, BoardName: h.lookupBoardName(c.Request.Context(), shell, boardUUID), EditMode: "draft"})
}

func (h *Handler) handleEditDraft(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "compose")
	if !ok {
		return
	}
	postUUID := strings.TrimSpace(c.Param("postUUID"))
	currentUser := shell.CurrentUser
	if currentUser == nil {
		h.redirectToLogin(c)
		return
	}
	draft, err := h.deps.PostUseCase.GetDraftPost(c.Request.Context(), postUUID, userIDFromModel(currentUser))
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	boardUUID := ""
	if draft != nil && draft.Post != nil {
		boardUUID = draft.Post.BoardUUID
	}
	var tagsInput string
	var titleInput string
	var contentInput string
	if draft != nil {
		if draft.Post != nil {
			titleInput = draft.Post.Title
			contentInput = draft.Post.Content
		}
		if len(draft.Tags) > 0 {
			items := make([]string, 0, len(draft.Tags))
			for _, tag := range draft.Tags {
				items = append(items, tag.Name)
			}
			tagsInput = strings.Join(items, ", ")
		}
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "compose", PostUUID: postUUID, BoardUUID: boardUUID, BoardName: h.lookupBoardName(c.Request.Context(), shell, boardUUID), PostDetail: draft, EditMode: "edit", TitleInput: titleInput, ContentInput: contentInput, TagsInput: tagsInput})
}

func (h *Handler) handleLoginPage(c *gin.Context) {
	if _, ok := h.currentUser(c); ok {
		c.Redirect(http.StatusSeeOther, "/me")
		return
	}
	shell, ok := h.shellForRequest(c, "profile")
	if !ok {
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "login", Redirect: safeRedirectTarget(c.Query("redirect")), Message: strings.TrimSpace(c.Query("message"))})
}

func (h *Handler) handleLoginSubmit(c *gin.Context) {
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	token, err := h.deps.SessionUseCase.Login(c.Request.Context(), username, password)
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.setSessionCookie(c, token, 0)
	redirect := safeRedirectTarget(c.PostForm("redirect"))
	if redirect == "/" {
		redirect = safeRedirectTarget(c.Query("redirect"))
	}
	c.Redirect(http.StatusSeeOther, redirect)
}

func (h *Handler) handleGuestLoginSubmit(c *gin.Context) {
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	if currentUser, ok := h.currentUser(c); ok && currentUser != nil {
		c.Redirect(http.StatusSeeOther, "/me")
		return
	}
	token, err := h.deps.SessionUseCase.IssueGuestToken(c.Request.Context())
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.setSessionCookie(c, token, guestSessionAge)
	redirect := safeRedirectTarget(c.PostForm("redirect"))
	if redirect == "/" {
		redirect = safeRedirectTarget(c.Query("redirect"))
	}
	c.Redirect(http.StatusSeeOther, redirect)
}

func (h *Handler) handleLogoutSubmit(c *gin.Context) {
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	if token, ok := h.authToken(c); ok {
		if err := h.deps.SessionUseCase.Logout(c.Request.Context(), token); err != nil {
			h.renderError(c, http.StatusInternalServerError, "Internal Server Error", err.Error())
			return
		}
	}
	h.clearSessionCookie(c)
	c.Redirect(http.StatusSeeOther, "/login")
}

func (h *Handler) handleSignupPage(c *gin.Context) {
	if _, ok := h.currentUser(c); ok {
		c.Redirect(http.StatusSeeOther, "/me")
		return
	}
	shell, ok := h.shellForRequest(c, "profile")
	if !ok {
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "signup", Redirect: safeRedirectTarget(c.Query("redirect"))})
}

func (h *Handler) handleSignupSubmit(c *gin.Context) {
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	username := strings.TrimSpace(c.PostForm("username"))
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	if _, err := h.deps.UserUseCase.SignUp(c.Request.Context(), username, email, password); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	redirect := safeRedirectTarget(c.PostForm("redirect"))
	if redirect == "/" {
		redirect = safeRedirectTarget(c.Query("redirect"))
	}
	loginURL := "/login?message=" + url.QueryEscape("Account created. Please log in.")
	if redirect != "/" {
		loginURL += "&redirect=" + url.QueryEscape(redirect)
	}
	c.Redirect(http.StatusSeeOther, loginURL)
}

func (h *Handler) handleVerifyEmailPage(c *gin.Context) {
	shell, ok := h.shellForRequest(c, "profile")
	if !ok {
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "verify-email", VerifyToken: strings.TrimSpace(c.Query("token"))})
}

func (h *Handler) handleVerifyEmailSubmit(c *gin.Context) {
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	token := strings.TrimSpace(c.PostForm("token"))
	if h.deps.AccountUseCase == nil {
		h.renderError(c, http.StatusNotImplemented, "Not Implemented", "account use case is not configured")
		return
	}
	if err := h.deps.AccountUseCase.ConfirmEmailVerification(c.Request.Context(), token); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/login?message="+url.QueryEscape("Email verified. Please log in."))
}

func (h *Handler) handleResetPasswordPage(c *gin.Context) {
	shell, ok := h.shellForRequest(c, "profile")
	if !ok {
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "reset-password", ResetToken: strings.TrimSpace(c.Query("token"))})
}

func (h *Handler) handleResetPasswordSubmit(c *gin.Context) {
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	if h.deps.AccountUseCase == nil {
		h.renderError(c, http.StatusNotImplemented, "Not Implemented", "account use case is not configured")
		return
	}
	token := strings.TrimSpace(c.PostForm("token"))
	if token != "" {
		newPassword := c.PostForm("new_password")
		if err := h.deps.AccountUseCase.ConfirmPasswordReset(c.Request.Context(), token, newPassword); err != nil {
			h.renderUseCaseError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, "/login?message="+url.QueryEscape("Password reset complete."))
	} else {
		email := strings.TrimSpace(c.PostForm("email"))
		if err := h.deps.AccountUseCase.RequestPasswordReset(c.Request.Context(), email); err != nil {
			h.renderUseCaseError(c, err)
			return
		}
		c.Redirect(http.StatusSeeOther, "/reset-password?message="+url.QueryEscape("Reset link sent to your email."))
	}
}

func (h *Handler) handleCreatePostSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "compose")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	boardUUID := strings.TrimSpace(c.Param("boardUUID"))
	postUUID, err := h.deps.PostUseCase.CreatePost(c.Request.Context(), c.PostForm("title"), c.PostForm("content"), parseTags(c.PostForm("tags")), nil, userIDFromModel(shell.CurrentUser), boardUUID)
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/posts/"+postUUID)
}

func (h *Handler) handleCreateDraftSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "compose")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	boardUUID := strings.TrimSpace(c.Param("boardUUID"))
	postUUID, err := h.deps.PostUseCase.CreateDraftPost(c.Request.Context(), c.PostForm("title"), c.PostForm("content"), parseTags(c.PostForm("tags")), nil, userIDFromModel(shell.CurrentUser), boardUUID)
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/posts/"+postUUID+"/edit")
}

func (h *Handler) handleUpdatePostSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "compose")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	postUUID := strings.TrimSpace(c.Param("postUUID"))
	if err := h.deps.PostUseCase.UpdatePost(c.Request.Context(), postUUID, userIDFromModel(shell.CurrentUser), c.PostForm("title"), c.PostForm("content"), parseTags(c.PostForm("tags"))); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/posts/"+postUUID+"/edit")
}

func (h *Handler) handlePublishPostSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "compose")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	postUUID := strings.TrimSpace(c.Param("postUUID"))
	if err := h.deps.PostUseCase.PublishPost(c.Request.Context(), postUUID, userIDFromModel(shell.CurrentUser)); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/posts/"+postUUID)
}

func (h *Handler) handleCreateCommentSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "feed")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	if h.deps.CommentUseCase == nil {
		h.renderError(c, http.StatusNotImplemented, "Not Implemented", "comment use case is not configured")
		return
	}
	postUUID := strings.TrimSpace(c.Param("postUUID"))
	if _, err := h.deps.CommentUseCase.CreateComment(c.Request.Context(), c.PostForm("content"), nil, userIDFromModel(shell.CurrentUser), postUUID, nil); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/posts/"+postUUID)
}

func (h *Handler) handleMePage(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "profile")
	if !ok {
		return
	}
	userID := userIDFromModel(shell.CurrentUser)
	limit := parsePageLimit(c.Query("limit"))
	page := parsePageNumber(c.Query("page"))
	drafts, currentPage, hasMore, err := loadSequentialPage(c.Request.Context(), page, "", func(ctx context.Context, cursor string) (*model.PostList, string, bool, error) {
		list, err := h.deps.PostUseCase.GetMyDraftPosts(ctx, userID, limit, cursor)
		if err != nil {
			return nil, "", false, err
		}
		nextCursor := ""
		if list != nil && list.NextCursor != nil {
			nextCursor = strings.TrimSpace(*list.NextCursor)
		}
		return list, nextCursor, list != nil && list.HasMore && nextCursor != "", nil
	})
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "me", ListBaseURL: "/me", ListLimit: limit, Pagination: buildPaginationData(currentPage, hasMore), Drafts: drafts})
}

func (h *Handler) handleMeDeleteSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "profile")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	password := c.PostForm("password")
	if err := h.deps.UserUseCase.DeleteMe(c.Request.Context(), userIDFromModel(shell.CurrentUser), password); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	_ = h.deps.SessionUseCase.InvalidateUserSessions(c.Request.Context(), userIDFromModel(shell.CurrentUser))
	h.clearSessionCookie(c)
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) handleMeRequestVerifyEmailSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "profile")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	if err := h.deps.AccountUseCase.RequestEmailVerification(c.Request.Context(), userIDFromModel(shell.CurrentUser)); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/me?message="+url.QueryEscape("Verification email sent."))
}

func (h *Handler) handleMeUpgradePage(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "profile")
	if !ok {
		return
	}
	if !shell.CurrentUser.Guest {
		c.Redirect(http.StatusSeeOther, "/me")
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "me-upgrade"})
}

func (h *Handler) handleMeUpgradeSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "profile")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	token, ok2 := h.authToken(c)
	if !ok2 {
		h.renderError(c, http.StatusUnauthorized, "Unauthorized", "session not found")
		return
	}
	username := strings.TrimSpace(c.PostForm("username"))
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	newToken, err := h.deps.AccountUseCase.UpgradeGuestAccount(c.Request.Context(), userIDFromModel(shell.CurrentUser), token, username, email, password)
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.setSessionCookie(c, newToken, 0)
	c.Redirect(http.StatusSeeOther, "/me?message="+url.QueryEscape("Account upgraded successfully."))
}

func (h *Handler) handleNotificationsPage(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "profile")
	if !ok {
		return
	}
	limit := parsePageLimit(c.Query("limit"))
	page := parsePageNumber(c.Query("page"))
	notifs, currentPage, hasMore, err := loadSequentialPage(c.Request.Context(), page, "", func(ctx context.Context, cursor string) (*model.NotificationList, string, bool, error) {
		list, err := h.deps.NotificationUseCase.GetMyNotifications(ctx, userIDFromModel(shell.CurrentUser), limit, cursor)
		if err != nil {
			return nil, "", false, err
		}
		nextCursor := ""
		if list != nil && list.NextCursor != nil {
			nextCursor = strings.TrimSpace(*list.NextCursor)
		}
		return list, nextCursor, list != nil && list.HasMore && nextCursor != "", nil
	})
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "notifications", ListBaseURL: "/notifications", ListLimit: limit, Pagination: buildPaginationData(currentPage, hasMore), Notifications: notifs})
}

func (h *Handler) handleNotificationsReadAllSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "profile")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	if err := h.deps.NotificationUseCase.MarkAllMyNotificationsRead(c.Request.Context(), userIDFromModel(shell.CurrentUser)); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/notifications")
}

func (h *Handler) handlePostReactionSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "feed")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	postUUID := strings.TrimSpace(c.Param("postUUID"))
	reactionTypeRaw := strings.TrimSpace(c.PostForm("reaction_type"))
	if h.deps.ReactionUseCase == nil {
		c.Redirect(http.StatusSeeOther, "/posts/"+postUUID)
		return
	}
	reactionType, ok := model.ParseReactionType(reactionTypeRaw)
	if !ok {
		c.Redirect(http.StatusSeeOther, "/posts/"+postUUID)
		return
	}
	if strings.TrimSpace(c.PostForm("_method")) == "delete" {
		_ = h.deps.ReactionUseCase.DeleteReaction(c.Request.Context(), userIDFromModel(shell.CurrentUser), postUUID, model.ReactionTargetPost)
	} else {
		_, _ = h.deps.ReactionUseCase.SetReaction(c.Request.Context(), userIDFromModel(shell.CurrentUser), postUUID, model.ReactionTargetPost, reactionType)
	}
	c.Redirect(http.StatusSeeOther, "/posts/"+postUUID)
}

func (h *Handler) handleCommentReactionSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "feed")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	commentUUID := strings.TrimSpace(c.Param("commentUUID"))
	postUUID := strings.TrimSpace(c.PostForm("post_uuid"))
	reactionTypeRaw := strings.TrimSpace(c.PostForm("reaction_type"))
	if h.deps.ReactionUseCase == nil {
		if postUUID == "" {
			postUUID = "/"
		} else {
			postUUID = "/posts/" + postUUID
		}
		c.Redirect(http.StatusSeeOther, postUUID)
		return
	}
	reactionType, ok := model.ParseReactionType(reactionTypeRaw)
	if !ok {
		if postUUID == "" {
			postUUID = "/"
		} else {
			postUUID = "/posts/" + postUUID
		}
		c.Redirect(http.StatusSeeOther, postUUID)
		return
	}
	if strings.TrimSpace(c.PostForm("_method")) == "delete" {
		_ = h.deps.ReactionUseCase.DeleteReaction(c.Request.Context(), userIDFromModel(shell.CurrentUser), commentUUID, model.ReactionTargetComment)
	} else {
		_, _ = h.deps.ReactionUseCase.SetReaction(c.Request.Context(), userIDFromModel(shell.CurrentUser), commentUUID, model.ReactionTargetComment, reactionType)
	}
	if postUUID == "" {
		postUUID = "/"
	} else {
		postUUID = "/posts/" + postUUID
	}
	c.Redirect(http.StatusSeeOther, postUUID)
}

func (h *Handler) handleAdminReportsPage(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	limit := parsePageLimit(c.Query("limit"))
	page := parsePageNumber(c.Query("page"))
	reports, currentPage, hasMore, err := loadSequentialPage(c.Request.Context(), page, int64(0), func(ctx context.Context, lastID int64) (*model.ReportList, int64, bool, error) {
		list, err := h.deps.ReportUseCase.GetReports(ctx, userIDFromModel(shell.CurrentUser), nil, limit, lastID)
		if err != nil {
			return nil, 0, false, err
		}
		nextLastID := int64(0)
		if list != nil && list.NextLastID != nil {
			nextLastID = *list.NextLastID
		}
		return list, nextLastID, list != nil && list.HasMore && nextLastID > 0, nil
	})
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "admin-reports", ListBaseURL: "/admin/reports", ListLimit: limit, Pagination: buildPaginationData(currentPage, hasMore), Reports: reports})
}

func (h *Handler) handleAdminReportResolveSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	reportID, err := strconv.ParseInt(c.Param("reportID"), 10, 64)
	if err != nil || reportID <= 0 {
		h.renderError(c, http.StatusBadRequest, "Bad Request", "invalid report id")
		return
	}
	status := model.ReportStatus(strings.TrimSpace(c.PostForm("status")))
	if status == "" {
		status = model.ReportStatusAccepted
	}
	if err := h.deps.ReportUseCase.ResolveReport(c.Request.Context(), userIDFromModel(shell.CurrentUser), reportID, status, c.PostForm("resolution_note")); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/reports")
}

func (h *Handler) handleAdminOutboxPage(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	limit := parsePageLimit(c.Query("limit"))
	page := parsePageNumber(c.Query("page"))
	outbox, currentPage, hasMore, err := loadSequentialPage(c.Request.Context(), page, "", func(ctx context.Context, lastID string) (*model.OutboxDeadMessageList, string, bool, error) {
		list, err := h.deps.OutboxAdminUseCase.GetDeadMessages(ctx, userIDFromModel(shell.CurrentUser), limit, lastID)
		if err != nil {
			return nil, "", false, err
		}
		nextLastID := ""
		if list != nil && list.NextLastID != nil {
			nextLastID = strings.TrimSpace(*list.NextLastID)
		}
		return list, nextLastID, list != nil && list.HasMore && nextLastID != "", nil
	})
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "admin-outbox", ListBaseURL: "/admin/outbox", ListLimit: limit, Pagination: buildPaginationData(currentPage, hasMore), Outbox: outbox})
}

func (h *Handler) handleAdminOutboxRequeueSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	if err := h.deps.OutboxAdminUseCase.RequeueDeadMessage(c.Request.Context(), userIDFromModel(shell.CurrentUser), strings.TrimSpace(c.Param("messageID"))); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/outbox")
}

func (h *Handler) handleAdminOutboxDiscardSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	if err := h.deps.OutboxAdminUseCase.DiscardDeadMessage(c.Request.Context(), userIDFromModel(shell.CurrentUser), strings.TrimSpace(c.Param("messageID"))); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/outbox")
}

func (h *Handler) handleAdminDashboard(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	adminID := userIDFromModel(shell.CurrentUser)
	reports, _ := h.deps.ReportUseCase.GetReports(c.Request.Context(), adminID, nil, webDefaultLimit, 0)
	outbox, _ := h.deps.OutboxAdminUseCase.GetDeadMessages(c.Request.Context(), adminID, webDefaultLimit, "")
	boards, _ := h.deps.BoardUseCase.GetAllBoards(c.Request.Context(), webDefaultLimit, "")
	hiddenCount := 0
	if boards != nil {
		for _, b := range boards.Boards {
			if b.Hidden {
				hiddenCount++
			}
		}
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "admin-snapshot", Reports: reports, Outbox: outbox, BoardHiddenCount: hiddenCount})
}

func (h *Handler) handleAdminBoardsPage(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	limit := parsePageLimit(c.Query("limit"))
	page := parsePageNumber(c.Query("page"))
	boards, currentPage, hasMore, err := loadSequentialPage(c.Request.Context(), page, "", func(ctx context.Context, cursor string) (*model.BoardList, string, bool, error) {
		list, err := h.deps.BoardUseCase.GetAllBoards(ctx, limit, cursor)
		if err != nil {
			return nil, "", false, err
		}
		nextCursor := ""
		if list != nil && list.NextCursor != nil {
			nextCursor = strings.TrimSpace(*list.NextCursor)
		}
		return list, nextCursor, list != nil && list.HasMore && nextCursor != "", nil
	})
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	visibleCount := 0
	hiddenCount := 0
	if boards != nil {
		for _, board := range boards.Boards {
			if board.Hidden {
				hiddenCount++
			} else {
				visibleCount++
			}
		}
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "admin-boards", ListBaseURL: "/admin/boards", ListLimit: limit, Pagination: buildPaginationData(currentPage, hasMore), AdminBoards: boards, BoardVisibleCount: visibleCount, BoardHiddenCount: hiddenCount})
}

func (h *Handler) handleAdminBoardCreateSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	if h.deps.BoardUseCase == nil {
		h.renderError(c, http.StatusNotImplemented, "Not Implemented", "board use case is not configured")
		return
	}
	if _, err := h.deps.BoardUseCase.CreateBoard(c.Request.Context(), userIDFromModel(shell.CurrentUser), name, description); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/boards")
}

func (h *Handler) handleAdminBoardEditPage(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	boardUUID := strings.TrimSpace(c.Param("boardUUID"))
	boards, err := h.deps.BoardUseCase.GetAllBoards(c.Request.Context(), 100, "")
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	var target *model.Board
	if boards != nil {
		for _, b := range boards.Boards {
			if b.UUID == boardUUID {
				bc := b
				target = &bc
				break
			}
		}
	}
	if target == nil {
		h.renderError(c, http.StatusNotFound, "Not Found", "board not found")
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "admin-board-edit", BoardUUID: boardUUID, AdminBoardTarget: target})
}

func (h *Handler) handleAdminBoardUpdateSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	boardUUID := strings.TrimSpace(c.Param("boardUUID"))
	name := strings.TrimSpace(c.PostForm("name"))
	description := strings.TrimSpace(c.PostForm("description"))
	if err := h.deps.BoardUseCase.UpdateBoard(c.Request.Context(), boardUUID, userIDFromModel(shell.CurrentUser), name, description); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/boards")
}

func (h *Handler) handleAdminBoardDeleteSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	boardUUID := strings.TrimSpace(c.Param("boardUUID"))
	if err := h.deps.BoardUseCase.DeleteBoard(c.Request.Context(), boardUUID, userIDFromModel(shell.CurrentUser)); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/boards")
}

func (h *Handler) handleAdminBoardVisibilitySubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	boardUUID := strings.TrimSpace(c.Param("boardUUID"))
	hidden := strings.EqualFold(strings.TrimSpace(c.PostForm("hidden")), "true") || c.PostForm("hidden") == "1"
	if err := h.deps.BoardUseCase.SetBoardVisibility(c.Request.Context(), boardUUID, userIDFromModel(shell.CurrentUser), hidden); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/admin/boards")
}

func (h *Handler) handleAdminSuspensionPage(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	suspension, err := h.deps.UserUseCase.GetUserSuspension(c.Request.Context(), userIDFromModel(shell.CurrentUser), strings.TrimSpace(c.Param("userUUID")))
	if err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	h.renderPage(c, http.StatusOK, PageData{Shell: shell, Kind: "admin-suspension", Suspension: suspension})
}

func (h *Handler) handleAdminSuspensionSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAdminShell(c)
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	userUUID := strings.TrimSpace(c.Param("userUUID"))
	duration := model.SuspensionDuration(strings.TrimSpace(c.PostForm("duration")))
	switch strings.ToLower(strings.TrimSpace(c.PostForm("_method"))) {
	case "delete", "unsuspend":
		if err := h.deps.UserUseCase.UnsuspendUser(c.Request.Context(), userIDFromModel(shell.CurrentUser), userUUID); err != nil {
			h.renderUseCaseError(c, err)
			return
		}
	default:
		if err := h.deps.UserUseCase.SuspendUser(c.Request.Context(), userIDFromModel(shell.CurrentUser), userUUID, c.PostForm("reason"), duration); err != nil {
			h.renderUseCaseError(c, err)
			return
		}
	}
	c.Redirect(http.StatusSeeOther, "/admin/reports")
}

func (h *Handler) handleDeletePostSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	postUUID := strings.TrimSpace(c.Param("postUUID"))
	if err := h.deps.PostUseCase.DeletePost(c.Request.Context(), postUUID, userIDFromModel(shell.CurrentUser)); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handler) handleUpdateCommentSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	commentUUID := strings.TrimSpace(c.Param("commentUUID"))
	content := strings.TrimSpace(c.PostForm("content"))
	postUUID := strings.TrimSpace(c.PostForm("post_uuid"))
	if err := h.deps.CommentUseCase.UpdateComment(c.Request.Context(), commentUUID, userIDFromModel(shell.CurrentUser), content); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	redirect := "/"
	if postUUID != "" {
		redirect = "/posts/" + postUUID
	}
	c.Redirect(http.StatusSeeOther, redirect)
}

func (h *Handler) handleDeleteCommentSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	commentUUID := strings.TrimSpace(c.Param("commentUUID"))
	postUUID := strings.TrimSpace(c.PostForm("post_uuid"))
	if err := h.deps.CommentUseCase.DeleteComment(c.Request.Context(), commentUUID, userIDFromModel(shell.CurrentUser)); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	redirect := "/"
	if postUUID != "" {
		redirect = "/posts/" + postUUID
	}
	c.Redirect(http.StatusSeeOther, redirect)
}

func (h *Handler) handleCreateReportSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	targetType := model.ReportTargetType(strings.TrimSpace(c.PostForm("target_type")))
	targetUUID := strings.TrimSpace(c.PostForm("target_uuid"))
	reasonCode := model.ReportReasonCode(strings.TrimSpace(c.PostForm("reason_code")))
	reasonDetail := strings.TrimSpace(c.PostForm("reason_detail"))
	redirect := strings.TrimSpace(c.PostForm("redirect"))
	if _, err := h.deps.ReportUseCase.CreateReport(c.Request.Context(), userIDFromModel(shell.CurrentUser), targetType, targetUUID, reasonCode, reasonDetail); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	if redirect == "" {
		redirect = "/"
	}
	c.Redirect(http.StatusSeeOther, redirect+"?message="+url.QueryEscape("Report submitted."))
}

func (h *Handler) handleNotificationReadSubmit(c *gin.Context) {
	shell, ok := h.requireHTMLAuthShell(c, "")
	if !ok {
		return
	}
	if err := h.requireCSRF(c); err != nil {
		h.renderError(c, http.StatusForbidden, "Forbidden", err.Error())
		return
	}
	notificationUUID := strings.TrimSpace(c.Param("notificationUUID"))
	if err := h.deps.NotificationUseCase.MarkMyNotificationRead(c.Request.Context(), userIDFromModel(shell.CurrentUser), notificationUUID); err != nil {
		h.renderUseCaseError(c, err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/notifications")
}

func (h *Handler) RenderError(c *gin.Context, status int, title, message string) {
	h.renderError(c, status, title, message)
}

func (h *Handler) RenderNotFound(c *gin.Context) {
	h.renderError(c, http.StatusNotFound, "Not Found", "page not found")
}

func (h *Handler) RenderMethodNotAllowed(c *gin.Context) {
	h.renderError(c, http.StatusMethodNotAllowed, "Method Not Allowed", "method not allowed")
}

func (h *Handler) renderUseCaseError(c *gin.Context, err error) {
	status := statusForError(err)
	h.renderError(c, status, http.StatusText(status), customerror.Public(err).Error())
}

func (h *Handler) renderPage(c *gin.Context, status int, data PageData) {
	data.Shell.Title = strings.TrimSpace(data.Shell.Title)
	if data.Shell.AppName == "" {
		data.Shell.AppName = fallbackAppName(h.deps.AppName)
	}
	if data.Shell.CSRFToken == "" {
		data.Shell.CSRFToken = h.ensureCSRFToken(c)
	}
	if data.Shell.CurrentUser != nil {
		data.Shell.IsAuthenticated = true
		data.Shell.IsAdmin = strings.EqualFold(data.Shell.CurrentUser.Role, "admin")
	}
	if data.Shell.BoardMap == nil {
		data.Shell.BoardMap = make(map[string]model.Board, len(data.Shell.Boards))
		for _, board := range data.Shell.Boards {
			data.Shell.BoardMap[board.UUID] = board
		}
	}
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, "layout", data); err != nil {
		h.renderError(c, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	c.Status(status)
	_, _ = c.Writer.Write(buf.Bytes())
}

func (h *Handler) renderError(c *gin.Context, status int, title, message string) {
	shell, _ := h.shellForRequest(c, "error")
	shell.Title = title
	h.renderPage(c, status, PageData{Shell: shell, Kind: "error", Message: message, ErrorMessage: message})
}

func (h *Handler) shellForRequest(c *gin.Context, activeNav string) (ShellData, bool) {
	user, ok := h.currentUser(c)
	_, hasToken := h.authToken(c)
	skipGuest := false
	if !ok && h.hasInvalidCookie(c) {
		h.clearSessionCookie(c)
		skipGuest = true
	}
	if !ok && !hasToken && !skipGuest && h.shouldAutoIssueGuest(activeNav) {
		if guest, issued := h.issueGuestUser(c); issued {
			user = guest
			ok = true
		}
	}
	boards := h.loadBoards(c.Request.Context())
	csrfToken := h.ensureCSRFToken(c)
	composeURL := ""
	if len(boards) > 0 {
		composeURL = "/boards/" + boards[0].UUID + "/posts/new"
	}
	shell := ShellData{
		AppName:     fallbackAppName(h.deps.AppName),
		Title:       fallbackAppName(h.deps.AppName),
		ActiveNav:   activeNav,
		ComposeURL:  composeURL,
		CurrentUser: user,
		UnreadCount: h.loadUnreadCount(c.Request.Context(), user),
		CSRFToken:   csrfToken,
		Boards:      boards,
	}
	if user != nil {
		shell.IsAuthenticated = true
		shell.IsAdmin = strings.EqualFold(user.Role, "admin")
	}
	return shell, true
}

func (h *Handler) shouldAutoIssueGuest(activeNav string) bool {
	switch activeNav {
	case "feed", "boards", "tags", "search":
		return true
	default:
		return false
	}
}

func (h *Handler) issueGuestUser(c *gin.Context) (*model.User, bool) {
	if h.deps.SessionUseCase == nil || h.deps.UserUseCase == nil {
		return nil, false
	}
	token, err := h.deps.SessionUseCase.IssueGuestToken(c.Request.Context())
	if err != nil {
		return nil, false
	}
	userID, err := h.deps.SessionUseCase.ValidateTokenToId(c.Request.Context(), token)
	if err != nil {
		return nil, false
	}
	user, err := h.deps.UserUseCase.GetMe(c.Request.Context(), userID)
	if err != nil {
		return nil, false
	}
	h.setSessionCookie(c, token, guestSessionAge)
	return user, true
}

func (h *Handler) requireHTMLAuthShell(c *gin.Context, activeNav string) (ShellData, bool) {
	shell, _ := h.shellForRequest(c, activeNav)
	if shell.CurrentUser == nil {
		h.redirectToLogin(c)
		return ShellData{}, false
	}
	return shell, true
}

func (h *Handler) requireHTMLAuth(fn gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		shell, ok := h.requireHTMLAuthShell(c, "profile")
		if !ok {
			return
		}
		_ = shell
		fn(c)
	}
}

func (h *Handler) requireHTMLAdminShell(c *gin.Context) (ShellData, bool) {
	shell, ok := h.requireHTMLAuthShell(c, "admin")
	if !ok {
		return ShellData{}, false
	}
	if !shell.IsAdmin {
		h.renderError(c, http.StatusForbidden, "Forbidden", "admin access is required")
		return ShellData{}, false
	}
	return shell, true
}

func (h *Handler) requireHTMLAdmin(fn gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		shell, ok := h.requireHTMLAdminShell(c)
		if !ok {
			return
		}
		_ = shell
		fn(c)
	}
}

func (h *Handler) currentUser(c *gin.Context) (*model.User, bool) {
	token, ok := h.authToken(c)
	if !ok || h.deps.SessionUseCase == nil || h.deps.UserUseCase == nil {
		return nil, false
	}
	userID, err := h.deps.SessionUseCase.ValidateTokenToId(c.Request.Context(), token)
	if err != nil {
		return nil, false
	}
	user, err := h.deps.UserUseCase.GetMe(c.Request.Context(), userID)
	if err != nil {
		return nil, false
	}
	return user, true
}

func (h *Handler) authToken(c *gin.Context) (string, bool) {
	if cookie, err := c.Request.Cookie(sessionCookieName); err == nil {
		token := strings.TrimSpace(cookie.Value)
		if token != "" {
			return token, true
		}
	}
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if header == "" {
		return "", false
	}
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func (h *Handler) hasInvalidCookie(c *gin.Context) bool {
	token, ok := h.authToken(c)
	if !ok || token == "" || h.deps.SessionUseCase == nil {
		return false
	}
	_, err := h.deps.SessionUseCase.ValidateTokenToId(c.Request.Context(), token)
	return errors.Is(err, customerror.ErrInvalidToken)
}

func (h *Handler) loadBoards(ctx context.Context) []model.Board {
	if h.deps.BoardUseCase == nil {
		return nil
	}
	list, err := h.deps.BoardUseCase.GetBoards(ctx, 100, "")
	if err != nil || list == nil {
		return nil
	}
	return list.Boards
}

func (h *Handler) lookupBoardName(ctx context.Context, shell ShellData, boardUUID string) string {
	if boardUUID == "" {
		return ""
	}
	if shell.BoardMap != nil {
		if board, ok := shell.BoardMap[boardUUID]; ok && strings.TrimSpace(board.Name) != "" {
			return board.Name
		}
	}
	if h.deps.BoardUseCase == nil {
		return boardUUID
	}
	list, err := h.deps.BoardUseCase.GetAllBoards(ctx, 1000, "")
	if err != nil || list == nil {
		return boardUUID
	}
	for _, board := range list.Boards {
		if board.UUID == boardUUID && strings.TrimSpace(board.Name) != "" {
			return board.Name
		}
	}
	return boardUUID
}

func (h *Handler) loadUnreadCount(ctx context.Context, user *model.User) int {
	if user == nil || h.deps.NotificationUseCase == nil {
		return 0
	}
	count, err := h.deps.NotificationUseCase.GetMyUnreadNotificationCount(ctx, userIDFromModel(user))
	if err != nil {
		return 0
	}
	return count
}

func (h *Handler) ensureCSRFToken(c *gin.Context) string {
	if cookie, err := c.Request.Cookie(csrfCookieName); err == nil {
		token := strings.TrimSpace(cookie.Value)
		if token != "" {
			return token
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		buf = []byte(uuid.NewString())
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	h.setCookie(c, csrfCookieName, token, false, 24*time.Hour)
	return token
}

func (h *Handler) requireCSRF(c *gin.Context) error {
	if strings.TrimSpace(c.GetHeader("Authorization")) != "" && c.Request.Method != http.MethodGet {
		return nil
	}
	cookie, err := c.Request.Cookie(csrfCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return errors.New("missing csrf token")
	}
	submitted := strings.TrimSpace(c.GetHeader(csrfHeaderName))
	if submitted == "" {
		submitted = strings.TrimSpace(c.PostForm("_csrf"))
	}
	if submitted == "" || submitted != strings.TrimSpace(cookie.Value) {
		return errors.New("invalid csrf token")
	}
	return nil
}

func (h *Handler) redirectToLogin(c *gin.Context) {
	redirect := c.Request.URL.RequestURI()
	if redirect == "" {
		redirect = "/"
	}
	c.Redirect(http.StatusSeeOther, "/login?redirect="+url.QueryEscape(redirect))
}

func (h *Handler) setSessionCookie(c *gin.Context, token string, maxAge time.Duration) {
	h.setCookie(c, sessionCookieName, token, true, maxAge)
}

func (h *Handler) clearSessionCookie(c *gin.Context) {
	cookie := &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0)}
	http.SetCookie(c.Writer, cookie)
}

func (h *Handler) setCookie(c *gin.Context, name, value string, httpOnly bool, maxAge time.Duration) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: httpOnly,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(c.Request),
	}
	if maxAge > 0 {
		cookie.MaxAge = int(maxAge / time.Second)
		cookie.Expires = time.Now().Add(maxAge)
	}
	http.SetCookie(c.Writer, cookie)
}

func isSecureRequest(req *http.Request) bool {
	if req == nil {
		return false
	}
	if req.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")), "https")
}

func parsePageLimit(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return webDefaultLimit
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return webDefaultLimit
	}
	if value > 100 {
		return 100
	}
	return value
}

func parsePageNumber(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return 1
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 1
	}
	return clampWebPageNumber(value)
}

func parseLastID(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func safeRedirectTarget(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u == nil {
		return "/"
	}
	if u.IsAbs() || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return "/"
	}
	return raw
}

func fallbackAppName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Commu Bin"
	}
	return value
}

func userIDFromModel(user *model.User) int64 {
	if user == nil {
		return 0
	}
	return user.ID
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, customerror.ErrUnauthorized), errors.Is(err, customerror.ErrMissingAuthHeader), errors.Is(err, customerror.ErrInvalidToken):
		return http.StatusUnauthorized
	case errors.Is(err, customerror.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, customerror.ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, customerror.ErrUserNotFound), errors.Is(err, customerror.ErrBoardNotFound), errors.Is(err, customerror.ErrPostNotFound), errors.Is(err, customerror.ErrTagNotFound), errors.Is(err, customerror.ErrReportNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func parseTags(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}
	return out
}
