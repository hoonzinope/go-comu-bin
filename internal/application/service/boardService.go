package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/cache/key"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var _ application.BoardUseCase = (*BoardService)(nil)

type BoardService struct {
	repository          application.Repository
	cache               application.Cache
	cachePolicy         appcache.Policy
	authorizationPolicy policy.AuthorizationPolicy
}

func NewBoardService(repository application.Repository, cache application.Cache, cachePolicy appcache.Policy) *BoardService {
	return &BoardService{
		repository:          repository,
		cache:               cache,
		cachePolicy:         cachePolicy,
		authorizationPolicy: policy.NewRoleAuthorizationPolicy(),
	}
}

func (s *BoardService) GetBoards(limit int, lastID int64) (*dto.BoardList, error) {
	cacheKey := key.BoardList(limit, lastID)
	value, err := s.cache.GetOrSetWithTTL(cacheKey, s.cachePolicy.ListTTLSeconds, func() (interface{}, error) {
		// 커서 기반 페이지네이션을 위해 1개 더 조회한다.
		fetchLimit := limit
		if limit > 0 {
			fetchLimit = limit + 1
		}

		boards, err := s.repository.BoardRepository.SelectBoardList(fetchLimit, lastID)
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

		return &dto.BoardList{
			Boards:     boards,
			Limit:      limit,
			LastID:     lastID,
			HasMore:    hasMore,
			NextLastID: nextLastID,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	list, ok := value.(*dto.BoardList)
	if !ok {
		return nil, customError.ErrInternalServerError
	}
	return list, nil
}

func (s *BoardService) CreateBoard(userID int64, name, description string) (int64, error) {
	// 게시판 생성 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(userID) // user 존재 여부 확인
	if user == nil || err != nil {
		return 0, customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.AdminOnly(user); err != nil {
		return 0, err
	}
	newBoard := entity.NewBoard(name, description)
	boardID, err := s.repository.BoardRepository.Save(newBoard)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(key.BoardListPrefix())
	return boardID, nil
}

func (s *BoardService) UpdateBoard(id, userID int64, name, description string) error {
	// 게시판 수정 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(userID) // user 존재 여부 확인
	if user == nil || err != nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.AdminOnly(user); err != nil {
		return err
	}
	existingBoard, err := s.repository.BoardRepository.SelectBoardByID(id) // board 존재 여부 확인
	if existingBoard == nil || err != nil {
		return customError.ErrInternalServerError
	}
	existingBoard.Update(name, description)
	err = s.repository.BoardRepository.Update(existingBoard)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(key.BoardListPrefix())
	return nil
}

func (s *BoardService) DeleteBoard(id, userID int64) error {
	// 게시판 삭제 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(userID) // user 존재 여부 확인
	if user == nil || err != nil {
		return customError.ErrUserNotFound
	}
	if err := s.authorizationPolicy.AdminOnly(user); err != nil {
		return err
	}
	existingBoard, err := s.repository.BoardRepository.SelectBoardByID(id) // board 존재 여부 확인
	if existingBoard == nil || err != nil {
		return customError.ErrInternalServerError
	}
	err = s.repository.BoardRepository.Delete(existingBoard.ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	s.cache.DeleteByPrefix(key.BoardListPrefix())
	return nil
}
