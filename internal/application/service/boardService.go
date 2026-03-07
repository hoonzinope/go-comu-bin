package service

import (
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
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
	cache               port.Cache
	cachePolicy         appcache.Policy
	authorizationPolicy policy.AuthorizationPolicy
}

func NewBoardService(userRepository port.UserRepository, boardRepository port.BoardRepository, cache port.Cache, cachePolicy appcache.Policy, authorizationPolicy policy.AuthorizationPolicy) *BoardService {
	return &BoardService{
		userRepository:      userRepository,
		boardRepository:     boardRepository,
		cache:               cache,
		cachePolicy:         cachePolicy,
		authorizationPolicy: authorizationPolicy,
	}
}

func (s *BoardService) GetBoards(limit int, lastID int64) (*model.BoardList, error) {
	cacheKey := key.BoardList(limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		// 커서 기반 페이지네이션을 위해 1개 더 조회한다.
		fetchLimit := limit
		if limit > 0 {
			fetchLimit = limit + 1
		}

		boards, err := s.boardRepository.SelectBoardList(fetchLimit, lastID)
		if err != nil {
			return nil, customError.ErrInternalServerError
		}

		hasMore := false
		var nextLastID *int64
		if limit >= 0 && len(boards) > limit {
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
		return nil, err
	}
	list, ok := value.(*model.BoardList)
	if !ok {
		return nil, customError.ErrInternalServerError
	}
	return list, nil
}

func (s *BoardService) CreateBoard(userID int64, name, description string) (int64, error) {
	// 게시판 생성 로직 구현
	user, err := s.userRepository.SelectUserByID(userID) // user 존재 여부 확인
	if user == nil || err != nil {
		return 0, customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.AdminOnly(user); err != nil {
		return 0, err
	}
	newBoard := entity.NewBoard(name, description)
	boardID, err := s.boardRepository.Save(newBoard)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(key.BoardListPrefix())
	return boardID, nil
}

func (s *BoardService) UpdateBoard(id, userID int64, name, description string) error {
	// 게시판 수정 로직 구현
	user, err := s.userRepository.SelectUserByID(userID) // user 존재 여부 확인
	if user == nil || err != nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.AdminOnly(user); err != nil {
		return err
	}
	existingBoard, err := s.boardRepository.SelectBoardByID(id) // board 존재 여부 확인
	if err != nil {
		return customError.ErrInternalServerError
	}
	if existingBoard == nil {
		return customError.ErrBoardNotFound
	}
	existingBoard.Update(name, description)
	err = s.boardRepository.Update(existingBoard)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(key.BoardListPrefix())
	return nil
}

func (s *BoardService) DeleteBoard(id, userID int64) error {
	// 게시판 삭제 로직 구현
	user, err := s.userRepository.SelectUserByID(userID) // user 존재 여부 확인
	if user == nil || err != nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.AdminOnly(user); err != nil {
		return err
	}
	existingBoard, err := s.boardRepository.SelectBoardByID(id) // board 존재 여부 확인
	if err != nil {
		return customError.ErrInternalServerError
	}
	if existingBoard == nil {
		return customError.ErrBoardNotFound
	}
	err = s.boardRepository.Delete(existingBoard.ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(key.BoardListPrefix())
	return nil
}
