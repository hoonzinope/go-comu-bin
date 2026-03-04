package policy

import "github.com/hoonzinope/go-comu-bin/internal/domain/entity"

type AuthorizationPolicy interface {
	AdminOnly(user *entity.User) error
	OwnerOrAdmin(user *entity.User, resourceOwnerID int64) error
}
