package service

import (
	"log/slog"

	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	outboxadminsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/outboxadmin"
)

type OutboxAdminService = outboxadminsvc.Service

func NewOutboxAdminService(userRepository port.UserRepository, outboxStore port.OutboxStore, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *OutboxAdminService {
	return outboxadminsvc.NewService(userRepository, outboxStore, authorizationPolicy, logger...)
}
