package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain"
)

type BoardService struct {
	repository application.Repository
}

func NewBoardService(repository application.Repository) *BoardService {
	return &BoardService{
		repository: repository,
	}
}

func (s *BoardService) GetBoards(limit, offset int) ([]*domain.Board, error) {
	// 게시판 목록 조회 로직 구현
	boards, err := s.repository.BoardRepository.SelectBoardList(limit, offset)
	if err != nil {
		return nil, customError.ErrInternalServerError
	}
	return boards, nil
}

func (s *BoardService) CreateBoard(userID int64, name, description string) (int64, error) {
	// 게시판 생성 로직 구현
	// TODO: only admin can create board
	boardID, err := s.repository.BoardRepository.SaveBoard(name, description)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	return boardID, nil
}
