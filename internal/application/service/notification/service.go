package notification

import (
	"context"
	"errors"

	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.NotificationUseCase = (*Service)(nil)

type Service struct {
	userRepository         port.UserRepository
	postRepository         port.PostRepository
	commentRepository      port.CommentRepository
	notificationRepository port.NotificationRepository
}

func NewService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, notificationRepository port.NotificationRepository) *Service {
	return &Service{
		userRepository:         userRepository,
		postRepository:         postRepository,
		commentRepository:      commentRepository,
		notificationRepository: notificationRepository,
	}
}

func (s *Service) GetMyNotifications(ctx context.Context, userID int64, limit int, cursor string) (*model.NotificationList, error) {
	if err := svccommon.RequirePositiveLimit(limit); err != nil {
		return nil, err
	}
	lastID, err := svccommon.DecodeOpaqueCursor(cursor)
	if err != nil {
		return nil, err
	}
	if err := s.ensureUserExists(ctx, userID); err != nil {
		return nil, err
	}
	fetchLimit, err := svccommon.CursorFetchLimit(limit)
	if err != nil {
		return nil, err
	}
	page, err := svccommon.LoadCursorListPage(ctx, limit, cursor, lastID, func(ctx context.Context) ([]*entity.Notification, error) {
		items, err := s.notificationRepository.SelectByRecipientUserID(ctx, userID, fetchLimit, lastID)
		if err != nil {
			return nil, customerror.WrapRepository("select notifications by recipient user id", err)
		}
		return items, nil
	}, func(item *entity.Notification) int64 {
		return item.ID
	})
	if err != nil {
		return nil, err
	}
	views, err := s.notificationsFromEntities(ctx, page.Items)
	if err != nil {
		return nil, err
	}
	return &model.NotificationList{
		Notifications: views,
		Limit:         limit,
		Cursor:        cursor,
		HasMore:       page.HasMore,
		NextCursor:    page.NextCursor,
	}, nil
}

func (s *Service) GetMyUnreadNotificationCount(ctx context.Context, userID int64) (int, error) {
	if err := s.ensureUserExists(ctx, userID); err != nil {
		return 0, err
	}
	count, err := s.notificationRepository.CountUnreadByRecipientUserID(ctx, userID)
	if err != nil {
		return 0, customerror.WrapRepository("count unread notifications by recipient user id", err)
	}
	return count, nil
}

func (s *Service) MarkMyNotificationRead(ctx context.Context, userID int64, notificationUUID string) error {
	if err := s.ensureUserExists(ctx, userID); err != nil {
		return err
	}
	notification, err := s.notificationRepository.SelectByUUID(ctx, notificationUUID)
	if err != nil {
		return customerror.WrapRepository("select notification by uuid", err)
	}
	if notification == nil || notification.RecipientUserID != userID {
		return customerror.ErrNotificationNotFound
	}
	if err := s.notificationRepository.MarkRead(ctx, notification.ID); err != nil {
		return customerror.WrapRepository("mark notification read", err)
	}
	return nil
}

func (s *Service) ensureUserExists(ctx context.Context, userID int64) error {
	user, err := s.userRepository.SelectUserByID(ctx, userID)
	if err != nil {
		return customerror.WrapRepository("select user by id for notification", err)
	}
	if user == nil {
		return customerror.ErrUserNotFound
	}
	return nil
}

func (s *Service) notificationsFromEntities(ctx context.Context, items []*entity.Notification) ([]model.Notification, error) {
	if len(items) == 0 {
		return []model.Notification{}, nil
	}
	userIDs := make([]int64, 0, len(items))
	postIDs := make([]int64, 0, len(items))
	commentIDs := make([]int64, 0, len(items))
	for _, item := range items {
		userIDs = append(userIDs, item.ActorUserID)
		if item.PostID > 0 {
			postIDs = append(postIDs, item.PostID)
		}
		if item.CommentID > 0 {
			commentIDs = append(commentIDs, item.CommentID)
		}
	}
	actorUUIDs, err := svccommon.UserUUIDsByIDs(ctx, s.userRepository, userIDs)
	if err != nil {
		return nil, err
	}
	postUUIDs, err := s.postRepository.SelectPostUUIDsByIDsIncludingDeleted(ctx, postIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select post uuids by ids including deleted", err)
	}
	commentUUIDs, err := s.commentRepository.SelectCommentUUIDsByIDsIncludingDeleted(ctx, commentIDs)
	if err != nil {
		return nil, customerror.WrapRepository("select comment uuids by ids including deleted", err)
	}
	views := make([]model.Notification, 0, len(items))
	for _, item := range items {
		actorUUID, ok := actorUUIDs[item.ActorUserID]
		if !ok {
			return nil, customerror.WrapRepository("select users by ids including deleted", errors.New("notification actor not found"))
		}
		postUUID := ""
		if item.PostID > 0 {
			postUUID = postUUIDs[item.PostID]
		}
		var commentUUID *string
		if item.CommentID > 0 {
			if value, ok := commentUUIDs[item.CommentID]; ok {
				commentUUID = &value
			}
		}
		views = append(views, model.Notification{
			UUID:           item.UUID,
			Type:           model.NotificationType(item.Type),
			ActorUUID:      actorUUID,
			PostUUID:       postUUID,
			CommentUUID:    commentUUID,
			ActorName:      item.ActorNameSnapshot,
			PostTitle:      item.PostTitleSnapshot,
			CommentPreview: item.CommentPreviewSnapshot,
			ReadAt:         item.ReadAt,
			CreatedAt:      item.CreatedAt,
		})
	}
	return views, nil
}
