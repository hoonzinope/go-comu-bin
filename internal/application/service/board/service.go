package board

import (
	"context"
	svccommon "github.com/hoonzinope/go-comu-bin/internal/application/service/common"
	"log/slog"
	"strings"

	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/mapper"
	"github.com/hoonzinope/go-comu-bin/internal/application/model"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	customerror "github.com/hoonzinope/go-comu-bin/internal/customerror"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ port.BoardUseCase = (*BoardService)(nil)

type Service = BoardService

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

func NewService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *Service {
	return NewBoardService(userRepository, boardRepository, postRepository, unitOfWork, cache, cachePolicy, authorizationPolicy, logger...)
}

func NewBoardServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *BoardService {
	return &BoardService{
		userRepository:      userRepository,
		boardRepository:     boardRepository,
		postRepository:      postRepository,
		unitOfWork:          unitOfWork,
		cache:               cache,
		actionDispatcher:    svccommon.ResolveActionDispatcher(actionDispatcher),
		cachePolicy:         cachePolicy,
		authorizationPolicy: authorizationPolicy,
		logger:              svccommon.ResolveLogger(logger),
	}
}

func NewServiceWithActionDispatcher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, cache port.Cache, actionDispatcher port.ActionHookDispatcher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...*slog.Logger) *Service {
	return NewBoardServiceWithActionDispatcher(userRepository, boardRepository, postRepository, unitOfWork, cache, actionDispatcher, cachePolicy, authorizationPolicy, logger...)
}

func (s *BoardService) GetBoards(ctx context.Context, limit int, cursor string) (*model.BoardList, error) {
	if err := svccommon.RequirePositiveLimit(limit); err != nil {
		return nil, err
	}
	lastID, err := svccommon.DecodeOpaqueCursor(cursor)
	if err != nil {
		return nil, err
	}
	cacheKey := key.BoardList(limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(ctx, cacheKey, s.cachePolicy.ListTTLSeconds, func(ctx context.Context) (interface{}, error) {
		fetchLimit, err := svccommon.CursorFetchLimit(limit)
		if err != nil {
			return nil, err
		}
		page, err := svccommon.LoadCursorListPage(ctx, limit, cursor, lastID, func(ctx context.Context) ([]*entity.Board, error) {
			visibleBoards, err := s.boardRepository.SelectBoardList(ctx, fetchLimit, lastID)
			if err != nil {
				return nil, customerror.WrapRepository("select board list", err)
			}
			return visibleBoards, nil
		}, func(item *entity.Board) int64 {
			return item.ID
		})
		if err != nil {
			return nil, err
		}

		return &model.BoardList{
			Boards:     mapper.BoardsFromEntities(page.Items),
			Limit:      limit,
			Cursor:     page.Cursor,
			HasMore:    page.HasMore,
			NextCursor: page.NextCursor,
		}, nil
	})
	if err != nil {
		return nil, svccommon.NormalizeCacheLoadError("load board list cache", err)
	}
	list, ok := value.(*model.BoardList)
	if !ok {
		return nil, customerror.Mark(customerror.ErrCacheFailure, "decode board list cache payload")
	}
	return list, nil
}

func (s *BoardService) CreateBoard(ctx context.Context, userID int64, name, description string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", customerror.ErrInvalidInput
	}
	newBoard := entity.NewBoard(name, description)
	var boardUUID string
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for create board", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		boardID, err := tx.BoardRepository().Save(txCtx, newBoard)
		if err != nil {
			return customerror.WrapRepository("save board", err)
		}
		boardUUID = newBoard.UUID
		if err := svccommon.DispatchDomainActions(tx, s.actionDispatcher, appevent.NewBoardChanged("created", boardID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return boardUUID, nil
}

func (s *BoardService) UpdateBoard(ctx context.Context, boardUUID string, userID int64, name, description string) error {
	if strings.TrimSpace(name) == "" {
		return customerror.ErrInvalidInput
	}
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for update board", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		existingBoard, err := tx.BoardRepository().SelectBoardByUUID(txCtx, boardUUID)
		if err != nil {
			return customerror.WrapRepository("select board by id for update board", err)
		}
		if existingBoard == nil {
			return customerror.ErrBoardNotFound
		}
		existingBoard.Update(name, description)
		if err := tx.BoardRepository().Update(txCtx, existingBoard); err != nil {
			return customerror.WrapRepository("update board", err)
		}
		if err := svccommon.DispatchDomainActions(tx, s.actionDispatcher, appevent.NewBoardChanged("updated", existingBoard.ID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *BoardService) DeleteBoard(ctx context.Context, boardUUID string, userID int64) error {
	err := s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for delete board", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		existingBoard, err := tx.BoardRepository().SelectBoardByUUID(txCtx, boardUUID)
		if err != nil {
			return customerror.WrapRepository("select board by id for delete board", err)
		}
		if existingBoard == nil {
			return customerror.ErrBoardNotFound
		}
		hasPosts, err := tx.PostRepository().ExistsByBoardID(txCtx, existingBoard.ID)
		if err != nil {
			return customerror.WrapRepository("check board posts before delete board", err)
		}
		if hasPosts {
			return customerror.ErrBoardNotEmpty
		}
		if err := tx.BoardRepository().Delete(txCtx, existingBoard.ID); err != nil {
			return customerror.WrapRepository("delete board", err)
		}
		if err := svccommon.DispatchDomainActions(tx, s.actionDispatcher, appevent.NewBoardChanged("deleted", existingBoard.ID)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *BoardService) SetBoardVisibility(ctx context.Context, boardUUID string, userID int64, hidden bool) error {
	return s.unitOfWork.WithinTransaction(ctx, func(tx port.TxScope) error {
		txCtx := tx.Context()
		user, err := tx.UserRepository().SelectUserByID(txCtx, userID)
		if err != nil {
			return customerror.WrapRepository("select user by id for set board visibility", err)
		}
		if user == nil {
			return customerror.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		existingBoard, err := tx.BoardRepository().SelectBoardByUUID(txCtx, boardUUID)
		if err != nil {
			return customerror.WrapRepository("select board by id for set board visibility", err)
		}
		if existingBoard == nil {
			return customerror.ErrBoardNotFound
		}
		existingBoard.SetHidden(hidden)
		if err := tx.BoardRepository().Update(txCtx, existingBoard); err != nil {
			return customerror.WrapRepository("update board visibility", err)
		}
		if err := svccommon.DispatchDomainActions(tx, s.actionDispatcher, appevent.NewBoardChanged("visibility", existingBoard.ID)); err != nil {
			return err
		}
		s.logger.Info("admin board visibility changed", "board_id", existingBoard.ID, "board_uuid", existingBoard.UUID, "hidden", hidden, "admin_id", userID)
		return nil
	})
}
