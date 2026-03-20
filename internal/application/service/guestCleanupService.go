package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	guestcleanupsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/guestcleanup"
)

type GuestCleanupService = guestcleanupsvc.Service

func NewGuestCleanupService(userRepository port.UserRepository, postRepository port.PostRepository, commentRepository port.CommentRepository, reactionRepository port.ReactionRepository, reportRepository port.ReportRepository, sessionRepository port.SessionRepository, unitOfWork port.UnitOfWork) *GuestCleanupService {
	return guestcleanupsvc.NewService(userRepository, postRepository, commentRepository, reactionRepository, reportRepository, sessionRepository, unitOfWork)
}
