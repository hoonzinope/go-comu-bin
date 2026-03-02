package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

type BoardService struct {
	repository application.Repository
}

func NewBoardService(repository application.Repository) *BoardService {
	return &BoardService{
		repository: repository,
	}
}

func (s *BoardService) GetBoards(limit, offset int) (*dto.BoardList, error) {
	// 게시판 목록 조회 로직 구현
	boards, err := s.repository.BoardRepository.SelectBoardList(limit, offset)
	if err != nil {
		return nil, customError.ErrInternalServerError
	}

	return &dto.BoardList{
		Boards: boards,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *BoardService) CreateBoard(userID int64, name, description string) (int64, error) {
	// 게시판 생성 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(userID) // user 존재 여부 확인
	if user == nil || err != nil {
		return 0, customError.ErrInternalServerError
	}
	newBoard := &entity.Board{}
	newBoard.NewBoard(name, description)
	boardID, err := s.repository.BoardRepository.Save(newBoard)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	return boardID, nil
}

func (s *BoardService) UpdateBoard(id, userID int64, name, description string) error {
	// 게시판 수정 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(userID) // user 존재 여부 확인
	if user == nil || err != nil {
		return customError.ErrInternalServerError
	}
	existingBoard, err := s.repository.BoardRepository.SelectBoardByID(id) // board 존재 여부 확인
	if existingBoard == nil || err != nil {
		return customError.ErrInternalServerError
	}
	existingBoard.UpdateBoard(name, description)
	err = s.repository.BoardRepository.Update(existingBoard)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}

func (s *BoardService) DeleteBoard(id, userID int64) error {
	// 게시판 삭제 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(userID) // user 존재 여부 확인
	if user == nil || err != nil {
		return customError.ErrInternalServerError
	}
	existingBoard, err := s.repository.BoardRepository.SelectBoardByID(id) // board 존재 여부 확인
	if existingBoard == nil || err != nil {
		return customError.ErrInternalServerError
	}
	err = s.repository.BoardRepository.Delete(existingBoard.ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}
