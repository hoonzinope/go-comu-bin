package service

import (
	"log/slog"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	boardsvc "github.com/hoonzinope/go-comu-bin/internal/application/service/board"
)

func NewBoardService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *boardsvc.BoardService {
	return boardsvc.NewBoardService(userRepository, boardRepository, postRepository, unitOfWork, cache, cachePolicy, authorizationPolicy, logger...)
}

func NewBoardServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *boardsvc.BoardService {
	return boardsvc.NewBoardServiceWithActionDispatcher(userRepository, boardRepository, postRepository, unitOfWork, cache, actionDispatcher, cachePolicy, authorizationPolicy, logger...)
}
