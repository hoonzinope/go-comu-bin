package port

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type TagRepository interface {
	Save(*entity.Tag) (int64, error)
	SelectByName(name string) (*entity.Tag, error)
	SelectByIDs(ids []int64) ([]*entity.Tag, error)
}
