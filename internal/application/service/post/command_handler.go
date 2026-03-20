package post

import (
	"context"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	"log/slog"
	"strings"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type postCommandHandler struct {
	boardRepository       port.BoardRepository
	postRepository        port.PostRepository
	unitOfWork            port.UnitOfWork
	actionDispatcher      port.ActionHookDispatcher
	authorizationPolicy   policy.AuthorizationPolicy
	logger                *slog.Logger
	tagCoordinator        *postTagCoordinator
	attachmentCoordinator *postAttachmentCoordinator
	deletionWorkflow      *postDeletionWorkflow
}

type CommandHandler = postCommandHandler

func newPostCommandHandler(boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, actionDispatcher port.ActionHookDispatcher, authorizationPolicy policy.AuthorizationPolicy, logger *slog.Logger, tagCoordinator *postTagCoordinator, attachmentCoordinator *postAttachmentCoordinator, deletionWorkflow *postDeletionWorkflow) *postCommandHandler {
	return &postCommandHandler{
		boardRepository:       boardRepository,
		postRepository:        postRepository,
		unitOfWork:            unitOfWork,
		actionDispatcher:      actionDispatcher,
		authorizationPolicy:   authorizationPolicy,
		logger:                logger,
		tagCoordinator:        tagCoordinator,
		attachmentCoordinator: attachmentCoordinator,
		deletionWorkflow:      deletionWorkflow,
	}
}

func NewCommandHandler(boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, actionDispatcher port.ActionHookDispatcher, authorizationPolicy policy.AuthorizationPolicy, logger *slog.Logger, tagCoordinator *TagCoordinator, attachmentCoordinator *AttachmentCoordinator, deletionWorkflow *DeletionWorkflow) *CommandHandler {
	return newPostCommandHandler(boardRepository, postRepository, unitOfWork, actionDispatcher, authorizationPolicy, logger, tagCoordinator, attachmentCoordinator, deletionWorkflow)
}

func (h *postCommandHandler) CreatePost(ctx context.Context, title, content string, tags []string, authorID int64, boardUUID string) (string, error) {
	return h.createPost(ctx, title, content, tags, authorID, boardUUID, false)
}

func (h *postCommandHandler) CreateDraftPost(ctx context.Context, title, content string, tags []string, authorID int64, boardUUID string) (string, error) {
	return h.createPost(ctx, title, content, tags, authorID, boardUUID, true)
}

func (h *postCommandHandler) createPost(ctx context.Context, title, content string, tags []string, authorID int64, boardUUID string, draft bool) (string, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return "", customerror.ErrInvalidInput
	}
	normalizedTags, err := normalizeTags(tags)
	if err != nil {
		return "", err
	}
	if len(extractAttachmentRefIDs(content)) > 0 {
		return "", customerror.ErrInvalidInput
	}
	board, err := h.boardRepository.SelectBoardByUUID(ctx, boardUUID)
	if err != nil {
		return "", customerror.WrapRepository("select board by uuid for create post", err)
	}
	if board == nil {
		return "", customerror.ErrBoardNotFound
	}
	var newPost *entity.Post
	if draft {
		newPost = entity.NewDraftPost(title, content, authorID, board.ID)
	} else {
		newPost = entity.NewPost(title, content, authorID, board.ID)
	}
	var postUUID string
	err = h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for create post", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		if draft {
			if err := policy.ForbidGuest(user); err != nil {
				return err
			}
		}
		if err := policy.EnsureGuestLifecycleAllowsWrite(user); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(user); err != nil {
			return err
		}
		board, err := tx.BoardRepository().SelectBoardByUUID(txCtx, boardUUID)
		if err != nil {
			return customerror.WrapRepository("select board by uuid for create post", err)
		}
		if board == nil {
			return customerror.ErrBoardNotFound
		}
		if err := policy.EnsureBoardVisible(board, user); err != nil {
			return err
		}
		postID, saveErr := tx.PostRepository().Save(txCtx, newPost)
		if saveErr != nil {
			return customerror.WrapRepository("save post", saveErr)
		}
		postUUID = newPost.UUID
		if err := h.tagCoordinator.upsertPostTags(tx, postID, normalizedTags); err != nil {
			return err
		}
		if !draft {
			if err := svccommon.DispatchDomainActions(tx, h.actionDispatcher, appevent.NewPostChanged("created", postID, board.ID, normalizedTags, nil)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return postUUID, nil
}

func (h *postCommandHandler) PublishPost(ctx context.Context, postUUID string, authorID int64) error {
	var boardID int64
	var postID int64
	var currentTags []string
	err := h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		post, err := tx.PostRepository().SelectPostByUUIDIncludingUnpublished(txCtx, postUUID)
		if err != nil {
			return customerror.WrapRepository("select post by id including unpublished for publish post", err)
		}
		if post == nil {
			return customerror.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for publish post", err)
		}
		if requester == nil {
			return customerror.ErrUserNotFound
		}
		if err := policy.ForbidGuest(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		if err := h.attachmentCoordinator.validateAttachmentRefsWithRepo(txCtx, tx.AttachmentRepository(), post.ID, post.Content); err != nil {
			return err
		}
		currentTags, err = h.tagCoordinator.activeTagNamesByPostIDTx(tx, post.ID)
		if err != nil {
			return err
		}
		if syncErr := h.attachmentCoordinator.syncPostAttachmentOrphans(txCtx, tx.AttachmentRepository(), post.ID, post.Content); syncErr != nil {
			return syncErr
		}
		publishedPost := *post
		publishedPost.Publish()
		if updateErr := tx.PostRepository().Update(txCtx, &publishedPost); updateErr != nil {
			return customerror.WrapRepository("publish post", updateErr)
		}
		boardID = post.BoardID
		postID = post.ID
		if err := svccommon.DispatchDomainActions(tx, h.actionDispatcher, appevent.NewPostChanged("published", postID, boardID, currentTags, nil)); err != nil {
			return err
		}
		return nil
	})
	return err
}

func (h *postCommandHandler) UpdatePost(ctx context.Context, postUUID string, authorID int64, title, content string, tags []string) error {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(content) == "" {
		return customerror.ErrInvalidInput
	}
	normalizedTags, err := normalizeTags(tags)
	if err != nil {
		return err
	}
	var postID, boardID int64
	var currentTagNames []string
	err = h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		post, err := tx.PostRepository().SelectPostByUUIDIncludingUnpublished(txCtx, postUUID)
		if err != nil {
			return customerror.WrapRepository("select post by id for update post", err)
		}
		if post == nil {
			return customerror.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for update post", err)
		}
		if requester == nil {
			return customerror.ErrUserNotFound
		}
		if err := policy.EnsureGuestLifecycleAllowsWrite(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		if err := h.attachmentCoordinator.validateAttachmentRefsWithRepo(txCtx, tx.AttachmentRepository(), post.ID, content); err != nil {
			return err
		}
		currentTagNames, err = h.tagCoordinator.activeTagNamesByPostIDTx(tx, post.ID)
		if err != nil {
			return err
		}
		if syncErr := h.attachmentCoordinator.syncPostAttachmentOrphans(txCtx, tx.AttachmentRepository(), post.ID, content); syncErr != nil {
			return syncErr
		}
		updatedPost := *post
		updatedPost.Update(title, content)
		if updateErr := tx.PostRepository().Update(txCtx, &updatedPost); updateErr != nil {
			return customerror.WrapRepository("update post", updateErr)
		}
		if err := h.tagCoordinator.syncPostTags(tx, post.ID, normalizedTags); err != nil {
			return err
		}
		postID = post.ID
		boardID = post.BoardID
		if err := svccommon.DispatchDomainActions(tx, h.actionDispatcher, appevent.NewPostChanged("updated", postID, boardID, unionTagNames(currentTagNames, normalizedTags), nil)); err != nil {
			return err
		}
		return nil
	})
	return err
}

func (h *postCommandHandler) DeletePost(ctx context.Context, postUUID string, authorID int64) error {
	var postID, boardID int64
	var currentTagNames []string
	var deletedCommentIDs []int64
	err := h.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		post, err := tx.PostRepository().SelectPostByUUIDIncludingUnpublished(txCtx, postUUID)
		if err != nil {
			return customerror.WrapRepository("select post by uuid for delete post", err)
		}
		if post == nil {
			return customerror.ErrPostNotFound
		}
		requester, err := tx.UserRepository().SelectUserByID(txCtx, authorID)
		if err != nil {
			return customerror.WrapRepository("select user by id for delete post", err)
		}
		if requester == nil {
			return customerror.ErrUserNotFound
		}
		if err := policy.EnsureGuestLifecycleAllowsWrite(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.CanWrite(requester); err != nil {
			return err
		}
		if err := h.authorizationPolicy.OwnerOrAdmin(requester, post.AuthorID); err != nil {
			return err
		}
		currentTagNames, err = h.tagCoordinator.activeTagNamesByPostIDTx(tx, post.ID)
		if err != nil {
			return err
		}
		deletedCommentIDs, err = h.deletionWorkflow.deletePostArtifacts(tx, post.ID)
		if err != nil {
			return err
		}
		if deleteErr := tx.PostRepository().Delete(txCtx, post.ID); deleteErr != nil {
			return customerror.WrapRepository("delete post", deleteErr)
		}
		if deleteErr := tx.PostTagRepository().SoftDeleteByPostID(txCtx, post.ID); deleteErr != nil {
			return customerror.WrapRepository("soft delete post tags", deleteErr)
		}
		postID = post.ID
		boardID = post.BoardID
		if err := svccommon.DispatchDomainActions(tx, h.actionDispatcher, appevent.NewPostChanged("deleted", postID, boardID, currentTagNames, deletedCommentIDs)); err != nil {
			return err
		}
		return nil
	})
	return err
}
