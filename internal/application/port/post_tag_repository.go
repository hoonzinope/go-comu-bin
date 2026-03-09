package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type PostTagRepository interface {
	SelectActiveByPostID(postID int64) ([]*entity.PostTag, error)
	SelectActiveByTagID(tagID int64, limit int, lastID int64) ([]*entity.PostTag, error)
	UpsertActive(postID, tagID int64) error
	SoftDelete(postID, tagID int64) error
	SoftDeleteByPostID(postID int64) error
}
