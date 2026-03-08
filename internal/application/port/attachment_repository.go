package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type AttachmentRepository interface {
	Save(*entity.Attachment) (int64, error)
	SelectByID(id int64) (*entity.Attachment, error)
	SelectByPostID(postID int64) ([]*entity.Attachment, error)
	Delete(id int64) error
}
