package service

import (
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
	eventPublisher      port.EventPublisher
	cachePolicy         appcache.Policy
	authorizationPolicy policy.AuthorizationPolicy
	logger              port.Logger
}

func NewBoardService(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...port.Logger) *BoardService {
	return NewBoardServiceWithPublisher(userRepository, boardRepository, postRepository, unitOfWork, cache, nil, cachePolicy, authorizationPolicy, logger...)
}

func NewBoardServiceWithPublisher(userRepository port.UserRepository, boardRepository port.BoardRepository, postRepository port.PostRepository, unitOfWork port.UnitOfWork, cache port.Cache, eventPublisher port.EventPublisher, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy, logger ...port.Logger) *BoardService {
	return &BoardService{
		userRepository:      userRepository,
		boardRepository:     boardRepository,
		postRepository:      postRepository,
		unitOfWork:          unitOfWork,
		cache:               cache,
		eventPublisher:      resolveEventPublisher(eventPublisher),
		cachePolicy:         cachePolicy,
		authorizationPolicy: authorizationPolicy,
		logger:              resolveLogger(logger),
	}
}

func (s *BoardService) GetBoards(limit int, lastID int64) (*model.BoardList, error) {
	if err := requirePositiveLimit(limit); err != nil {
		return nil, err
	}
	cacheKey := key.BoardList(limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		// 커서 기반 페이지네이션을 위해 1개 더 조회한다.
		fetchLimit := limit
		if limit > 0 {
			fetchLimit = limit + 1
		}

		boards, err := s.boardRepository.SelectBoardList(fetchLimit, lastID)
		if err != nil {
			return nil, customError.WrapRepository("select board list", err)
		}

		hasMore := false
		var nextLastID *int64
		if len(boards) > limit {
			hasMore = true
			boards = boards[:limit]
		}
		if hasMore && len(boards) > 0 {
			next := boards[len(boards)-1].ID
			nextLastID = &next
		}

		return &model.BoardList{
			Boards:     mapper.BoardsFromEntities(boards),
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

func (s *BoardService) CreateBoard(userID int64, name, description string) (int64, error) {
	// 게시판 생성 로직 구현
	if strings.TrimSpace(name) == "" {
		return 0, customError.ErrInvalidInput
	}
	newBoard := entity.NewBoard(name, description)
	var boardID int64
	err := s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		user, err := tx.UserRepository().SelectUserByID(userID)
		if err != nil {
			return customError.WrapRepository("select user by id for create board", err)
		}
		if user == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		boardID, err = tx.BoardRepository().Save(newBoard)
		if err != nil {
			return customError.WrapRepository("save board", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	s.eventPublisher.Publish(appevent.NewBoardChanged("created", boardID))
	return boardID, nil
}

func (s *BoardService) UpdateBoard(id, userID int64, name, description string) error {
	// 게시판 수정 로직 구현
	if strings.TrimSpace(name) == "" {
		return customError.ErrInvalidInput
	}
	err := s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		user, err := tx.UserRepository().SelectUserByID(userID)
		if err != nil {
			return customError.WrapRepository("select user by id for update board", err)
		}
		if user == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		existingBoard, err := tx.BoardRepository().SelectBoardByID(id)
		if err != nil {
			return customError.WrapRepository("select board by id for update board", err)
		}
		if existingBoard == nil {
			return customError.ErrBoardNotFound
		}
		existingBoard.Update(name, description)
		if err := tx.BoardRepository().Update(existingBoard); err != nil {
			return customError.WrapRepository("update board", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.eventPublisher.Publish(appevent.NewBoardChanged("updated", id))
	return nil
}

func (s *BoardService) DeleteBoard(id, userID int64) error {
	// 게시판 삭제 로직 구현
	err := s.unitOfWork.WithinTransaction(func(tx port.TxScope) error {
		user, err := tx.UserRepository().SelectUserByID(userID)
		if err != nil {
			return customError.WrapRepository("select user by id for delete board", err)
		}
		if user == nil {
			return customError.ErrUserNotFound
		}
		if err := s.authorizationPolicy.AdminOnly(user); err != nil {
			return err
		}
		existingBoard, err := tx.BoardRepository().SelectBoardByID(id)
		if err != nil {
			return customError.WrapRepository("select board by id for delete board", err)
		}
		if existingBoard == nil {
			return customError.ErrBoardNotFound
		}
		hasPosts, err := tx.PostRepository().ExistsByBoardID(existingBoard.ID)
		if err != nil {
			return customError.WrapRepository("check board posts before delete board", err)
		}
		if hasPosts {
			return customError.ErrBoardNotEmpty
		}
		if err := tx.BoardRepository().Delete(existingBoard.ID); err != nil {
			return customError.WrapRepository("delete board", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.eventPublisher.Publish(appevent.NewBoardChanged("deleted", id))
	return nil
}
