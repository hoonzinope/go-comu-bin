package common

import (
	"context"
	"sort"
	"strings"

	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

const (
	notificationPostTitleLimit      = 50
	notificationCommentPreviewLimit = 50
)

func BuildMentionNotificationEvents(ctx context.Context, userRepository port.UserRepository, actor *entity.User, mentionedUsernames []string, postID, commentID int64, postTitle, content string) ([]port.DomainEvent, error) {
	if actor == nil {
		return nil, nil
	}
	mentions := normalizeMentionedUsernames(mentionedUsernames)
	if len(mentions) == 0 {
		return nil, nil
	}
	commentPreview := TruncateNotificationSnapshot(content, notificationCommentPreviewLimit)
	postTitle = TruncateNotificationSnapshot(postTitle, notificationPostTitleLimit)
	events := make([]port.DomainEvent, 0, len(mentions))
	for _, username := range mentions {
		user, err := userRepository.SelectUserByUsername(ctx, username)
		if err != nil {
			return nil, customerror.WrapRepository("select user by username for mention notification", err)
		}
		if user == nil || user.ID == actor.ID {
			continue
		}
		events = append(events, appevent.NewNotificationTriggered(
			user.ID,
			actor.ID,
			entity.NotificationTypeMentioned,
			postID,
			commentID,
			actor.Name,
			postTitle,
			commentPreview,
		))
	}
	return events, nil
}

func normalizeMentionedUsernames(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name == "" {
			continue
		}
		seen[name] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
