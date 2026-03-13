package port

import (
	"context"

	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type TagRepository interface {
	Save(ctx context.Context, tag *entity.Tag) (int64, error)
	SelectByName(ctx context.Context, name string) (*entity.Tag, error)
	SelectByIDs(ctx context.Context, ids []int64) ([]*entity.Tag, error)
}
