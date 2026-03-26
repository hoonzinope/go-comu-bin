package sqlite

import (
	"database/sql"
	"time"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func intToBool(value int64) bool {
	return value != 0
}

func scanBoard(scanner rowScanner) (*entity.Board, error) {
	var hiddenInt int64
	var createdAt sql.NullInt64
	board := &entity.Board{}
	if err := scanner.Scan(&board.ID, &board.UUID, &board.Name, &board.Description, &hiddenInt, &createdAt); err != nil {
		return nil, err
	}
	board.Hidden = intToBool(hiddenInt)
	board.CreatedAt = mustParseSQLTimestamp("boards.created_at", createdAt)
	return board, nil
}

func scanTag(scanner rowScanner) (*entity.Tag, error) {
	var createdAt sql.NullInt64
	tag := &entity.Tag{}
	if err := scanner.Scan(&tag.ID, &tag.Name, &createdAt); err != nil {
		return nil, err
	}
	tag.CreatedAt = mustParseSQLTimestamp("tags.created_at", createdAt)
	return tag, nil
}

func scanPost(scanner rowScanner) (*entity.Post, error) {
	var publishedAt sql.NullInt64
	var deletedAt sql.NullInt64
	var createdAt sql.NullInt64
	var updatedAt sql.NullInt64
	post := &entity.Post{}
	if err := scanner.Scan(
		&post.ID,
		&post.UUID,
		&post.Title,
		&post.Content,
		&post.AuthorID,
		&post.BoardID,
		&post.Status,
		&createdAt,
		&publishedAt,
		&updatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	post.CreatedAt = mustParseSQLTimestamp("posts.created_at", createdAt)
	post.UpdatedAt = mustParseSQLTimestamp("posts.updated_at", updatedAt)
	post.PublishedAt = unixNanoToTimePtr(publishedAt)
	post.DeletedAt = unixNanoToTimePtr(deletedAt)
	return post, nil
}

func scanPostTag(scanner rowScanner) (*entity.PostTag, error) {
	var createdAt sql.NullInt64
	postTag := &entity.PostTag{}
	if err := scanner.Scan(&postTag.PostID, &postTag.TagID, &createdAt, &postTag.Status); err != nil {
		return nil, err
	}
	postTag.CreatedAt = mustParseSQLTimestamp("post_tags.created_at", createdAt)
	return postTag, nil
}

func scanComment(scanner rowScanner) (*entity.Comment, error) {
	var parentID sql.NullInt64
	var deletedAt sql.NullInt64
	var createdAt sql.NullInt64
	var updatedAt sql.NullInt64
	comment := &entity.Comment{}
	if err := scanner.Scan(
		&comment.ID,
		&comment.UUID,
		&comment.Content,
		&comment.AuthorID,
		&comment.PostID,
		&parentID,
		&comment.Status,
		&createdAt,
		&updatedAt,
		&deletedAt,
	); err != nil {
		return nil, err
	}
	comment.ParentID = nullableInt64ToPtr(parentID)
	comment.CreatedAt = mustParseSQLTimestamp("comments.created_at", createdAt)
	comment.UpdatedAt = mustParseSQLTimestamp("comments.updated_at", updatedAt)
	comment.DeletedAt = unixNanoToTimePtr(deletedAt)
	return comment, nil
}

func scanReaction(scanner rowScanner) (*entity.Reaction, error) {
	var createdAt sql.NullInt64
	reaction := &entity.Reaction{}
	if err := scanner.Scan(
		&reaction.ID,
		&reaction.TargetType,
		&reaction.TargetID,
		&reaction.Type,
		&reaction.UserID,
		&createdAt,
	); err != nil {
		return nil, err
	}
	reaction.CreatedAt = mustParseSQLTimestamp("reactions.created_at", createdAt)
	return reaction, nil
}

func scanAttachment(scanner rowScanner) (*entity.Attachment, error) {
	var orphanedAt sql.NullInt64
	var pendingDeleteAt sql.NullInt64
	var createdAt sql.NullInt64
	attachment := &entity.Attachment{}
	if err := scanner.Scan(
		&attachment.ID,
		&attachment.UUID,
		&attachment.PostID,
		&attachment.FileName,
		&attachment.ContentType,
		&attachment.SizeBytes,
		&attachment.StorageKey,
		&createdAt,
		&orphanedAt,
		&pendingDeleteAt,
	); err != nil {
		return nil, err
	}
	attachment.CreatedAt = mustParseSQLTimestamp("attachments.created_at", createdAt)
	attachment.OrphanedAt = unixNanoToTimePtr(orphanedAt)
	attachment.PendingDeleteAt = unixNanoToTimePtr(pendingDeleteAt)
	return attachment, nil
}

func scanReport(scanner rowScanner) (*entity.Report, error) {
	var resolvedBy sql.NullInt64
	var resolvedAt sql.NullInt64
	var createdAt sql.NullInt64
	var updatedAt sql.NullInt64
	report := &entity.Report{}
	if err := scanner.Scan(
		&report.ID,
		&report.TargetType,
		&report.TargetID,
		&report.ReporterUserID,
		&report.ReasonCode,
		&report.ReasonDetail,
		&report.Status,
		&report.ResolutionNote,
		&resolvedBy,
		&resolvedAt,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	report.ResolvedBy = nullableInt64ToPtr(resolvedBy)
	report.ResolvedAt = unixNanoToTimePtr(resolvedAt)
	report.CreatedAt = mustParseSQLTimestamp("reports.created_at", createdAt)
	report.UpdatedAt = mustParseSQLTimestamp("reports.updated_at", updatedAt)
	return report, nil
}

func scanNotification(scanner rowScanner) (*entity.Notification, error) {
	var readAt sql.NullInt64
	var createdAt sql.NullInt64
	var dedupKey sql.NullString
	notification := &entity.Notification{}
	if err := scanner.Scan(
		&notification.ID,
		&notification.UUID,
		&notification.RecipientUserID,
		&notification.ActorUserID,
		&notification.Type,
		&notification.PostID,
		&notification.CommentID,
		&notification.ActorNameSnapshot,
		&notification.PostTitleSnapshot,
		&notification.CommentPreviewSnapshot,
		&readAt,
		&createdAt,
		&dedupKey,
	); err != nil {
		return nil, err
	}
	notification.ReadAt = unixNanoToTimePtr(readAt)
	notification.CreatedAt = mustParseSQLTimestamp("notifications.created_at", createdAt)
	notification.DedupKey = dedupKey.String
	return notification, nil
}

func cloneBoard(board *entity.Board) *entity.Board {
	if board == nil {
		return nil
	}
	out := *board
	return &out
}

func cloneTag(tag *entity.Tag) *entity.Tag {
	if tag == nil {
		return nil
	}
	out := *tag
	return &out
}

func clonePost(post *entity.Post) *entity.Post {
	if post == nil {
		return nil
	}
	out := *post
	if post.PublishedAt != nil {
		publishedAt := *post.PublishedAt
		out.PublishedAt = &publishedAt
	}
	if post.DeletedAt != nil {
		deletedAt := *post.DeletedAt
		out.DeletedAt = &deletedAt
	}
	return &out
}

func clonePostTag(postTag *entity.PostTag) *entity.PostTag {
	if postTag == nil {
		return nil
	}
	out := *postTag
	return &out
}

func cloneComment(comment *entity.Comment) *entity.Comment {
	if comment == nil {
		return nil
	}
	out := *comment
	if comment.ParentID != nil {
		parentID := *comment.ParentID
		out.ParentID = &parentID
	}
	if comment.DeletedAt != nil {
		deletedAt := *comment.DeletedAt
		out.DeletedAt = &deletedAt
	}
	return &out
}

func cloneReaction(reaction *entity.Reaction) *entity.Reaction {
	if reaction == nil {
		return nil
	}
	out := *reaction
	return &out
}

func cloneAttachment(attachment *entity.Attachment) *entity.Attachment {
	if attachment == nil {
		return nil
	}
	out := *attachment
	if attachment.OrphanedAt != nil {
		orphanedAt := *attachment.OrphanedAt
		out.OrphanedAt = &orphanedAt
	}
	if attachment.PendingDeleteAt != nil {
		pendingDeleteAt := *attachment.PendingDeleteAt
		out.PendingDeleteAt = &pendingDeleteAt
	}
	return &out
}

func cloneReport(report *entity.Report) *entity.Report {
	if report == nil {
		return nil
	}
	out := *report
	if report.ResolvedBy != nil {
		resolvedBy := *report.ResolvedBy
		out.ResolvedBy = &resolvedBy
	}
	if report.ResolvedAt != nil {
		resolvedAt := *report.ResolvedAt
		out.ResolvedAt = &resolvedAt
	}
	return &out
}

func cloneNotification(notification *entity.Notification) *entity.Notification {
	if notification == nil {
		return nil
	}
	out := *notification
	if notification.ReadAt != nil {
		readAt := *notification.ReadAt
		out.ReadAt = &readAt
	}
	return &out
}

func timeToUnixNanoOrNull(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UnixNano()
}

func nullableInt64ToPtr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	out := value.Int64
	return &out
}
