package service

import (
	"context"
	"log/slog"
	"strings"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.BoardUseCase = (*BoardService)(nil)

type BoardService struct {
	userRepository      port.UserRepository
	boardRepository     port.BoardRepository
	postRepository      port.PostRepository
	unitOfWork          port.UnitOfWork
	cache               port.Cache
	actionDispatcher    port.ActionHookDispatcher
	cachePolicy         appcache.Policy
	authorizationPolicy policy.AuthorizationPolicy
	logger              *slog.Logger
}

func NewBoardService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *BoardService {
	return NewBoardServiceWithActionDispatcher(userRepository, boardRepository, postRepository, unitOfWork, cache, nil, cachePolicy, authorizationPolicy, logger...)
}

func NewBoardServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *BoardService {
	return &BoardService{
		userRepository:      userRepository,
		boardRepository:     boardRepository,
		postRepository:      postRepository,
		unitOfWork:          unitOfWork,
		cache:               cache,
		actionDispatcher:    resolveActionDispatcher(actionDispatcher),
		cachePolicy:         cachePolicy,
		authorizationPolicy: authorizationPolicy,
		logger:              resolveLogger(logger),
	}
}

func (s *BoardService) GetBoards(ctx context.Context, limit int, lastID int64) (*model.BoardList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	cacheKey := key.BoardList(limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(ctx, cacheKey, s.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		fetchLimit, err := cursorFetchLimit(limit)
		if err != nil {
			return nil, err
		}

		visibleBoards, err := s.boardRepository.SelectBoardList(ctx, fetchLimit, lastID)
		if err != nil {
			return nil, customError.WrapRepository("select board list", err)
		}

		hasMore := false
		var nextLastID *int64
		if len(visibleBoards) > limit {
			hasMore = true
			visibleBoards = visibleBoards[:limit]
		}
		if hasMore && len(visibleBoards) > 0 {
			next := visibleBoards[len(visibleBoards)-1].ID
			nextLastID = &next
		}

		return &model.BoardList{
			Boards:     mapper.BoardsFromEntities(visibleBoards),
			Limit:      limit,
			LastID:     lastID,
			HasMore:    hasMore,
			NextLastID: nextLastID,
		}, nil
	})
	if err != nil {
		return nil, normalizeCacheLoadError("load board list cache", err)
	}
	list, ok := value.(*model.BoardList)
	if !ok {
		return nil, customError.Mark(customError.ErrCacheFailure, "decode board list cache payload")
	}
	return list, nil
}

func (s *BoardService) CreateBoard(ctx context.Context, userID int64, name, description string) (int64, error) {
	// 게시판 생성 로직 구현
	if strings.TrimSpace(name) == "" {
		return 0, customError.ErrInvalidInput
	}
	newBoard := entity.NewBoard(name, description)
	var boardID int64
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customError.WrapRepository("select user by id for create board", err)
		}
		if user == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		boardID, err = tx.BoardRepository().Save(txCtx, newBoard)
		if err != nil {
			return customError.WrapRepository("save board", err)
		}
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewBoardChanged("created", boardID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return boardID, nil
}

func (s *BoardService) UpdateBoard(ctx context.Context, id, userID int64, name, description string) error {
	// 게시판 수정 로직 구현
	if strings.TrimSpace(name) == "" {
		return customError.ErrInvalidInput
	}
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customError.WrapRepository("select user by id for update board", err)
		}
		if user == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		existingBoard, err := tx.BoardRepository().SelectBoardByID(txCtx, id)
		if err != nil {
			return customError.WrapRepository("select board by id for update board", err)
		}
		if existingBoard == nil {
			return customError.ErrBoardNotFound
		}
		existingBoard.Update(name, description)
		if err := tx.BoardRepository().Update(txCtx, existingBoard); err != nil {
			return customError.WrapRepository("update board", err)
		}
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewBoardChanged("updated", id)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *BoardService) DeleteBoard(ctx context.Context, id, userID int64) error {
	// 게시판 삭제 로직 구현
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customError.WrapRepository("select user by id for delete board", err)
		}
		if user == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		existingBoard, err := tx.BoardRepository().SelectBoardByID(txCtx, id)
		if err != nil {
			return customError.WrapRepository("select board by id for delete board", err)
		}
		if existingBoard == nil {
			return customError.ErrBoardNotFound
		}
		hasPosts, err := tx.PostRepository().ExistsByBoardID(txCtx, existingBoard.ID)
		if err != nil {
			return customError.WrapRepository("check board posts before delete board", err)
		}
		if hasPosts {
			return customError.ErrBoardNotEmpty
		}
		if err := tx.BoardRepository().Delete(txCtx, existingBoard.ID); err != nil {
			return customError.WrapRepository("delete board", err)
		}
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewBoardChanged("deleted", id)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *BoardService) SetBoardVisibility(ctx context.Context, id, userID int64, hidden bool) error {
	return s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customError.WrapRepository("select user by id for set board visibility", err)
		}
		if user == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		existingBoard, err := tx.BoardRepository().SelectBoardByID(txCtx, id)
		if err != nil {
			return customError.WrapRepository("select board by id for set board visibility", err)
		}
		if existingBoard == nil {
			return customError.ErrBoardNotFound
		}
		existingBoard.SetHidden(hidden)
		if err := tx.BoardRepository().Update(txCtx, existingBoard); err != nil {
			return customError.WrapRepository("update board visibility", err)
		}
		if err := dispatchDomainActions(tx, s.actionDispatcher, appevent.NewBoardChanged("visibility", id)); err != nil {
			return err
		}
		s.logger.Info("admin board visibility changed", "board_id", id, "hidden", hidden, "admin_id", userID)
		return nil
	})
}
