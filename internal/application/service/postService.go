package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
)

type PostService struct {
	repository application.Repository
}

func NewPostService(repository application.Repository) *PostService {
	return &PostService{
		repository: repository,
	}
}

func (s *PostService) CreatePost(title, content string, authorID, boardID int64) (int64, error) {
	// 게시글 생성 로직 구현
	user, err := s.repository.UserRepository.SelectUserByID(authorID) // user 존재 여부 확인
	if user == nil || err != nil {
		return 0, customError.ErrInternalServerError
	}
	board, err := s.repository.BoardRepository.SelectBoardByID(boardID) // board 존재 여부 확인
	if board == nil || err != nil {
		return 0, customError.ErrInternalServerError
	}
	postID, err := s.repository.PostRepository.SavePost(title, content, authorID, boardID)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	return postID, nil
}

func (s *PostService) GetPostsByBoard(boardID int64, limit, offset int) ([]*dto.PostDetail, error) {
	// 게시글 목록 조회 로직 구현
	posts, err := s.repository.PostRepository.SelectPostsByBoardID(boardID, limit, offset)
	if err != nil {
		return nil, customError.ErrInternalServerError
	}
	postDetails := make([]*dto.PostDetail, len(posts))
	for i, post := range posts {
		reactions, err := s.repository.ReactionRepository.GetReactionsByTarget(post.ID, "post")
		if err != nil {
			return nil, customError.ErrInternalServerError
		}
		postDetails[i] = &dto.PostDetail{
			Post:      post,
			Reactions: reactions,
		}
	}
	return postDetails, nil
}

func (s *PostService) GetPostDetail(id int64) (*dto.PostDetail, error) {
	post, err := s.repository.PostRepository.SelectPostByID(id)
	if post == nil || err != nil {
		return nil, customError.ErrInternalServerError
	}
	reactions, err := s.repository.ReactionRepository.GetReactionsByTarget(post.ID, "post")
	if err != nil {
		return nil, customError.ErrInternalServerError
	}
	comments, err := s.repository.CommentRepository.SelectCommentsByPostID(post.ID, 100, 0) // 댓글은 최대 100개까지 조회
	commentDetails := make([]*dto.CommentDetail, len(comments))
	if err != nil {
		return nil, customError.ErrInternalServerError
	}
	for i, comment := range comments {
		commentReactions, err := s.repository.ReactionRepository.GetReactionsByTarget(comment.ID, "comment")
		if err != nil {
			return nil, customError.ErrInternalServerError
		}
		commentDetails[i] = &dto.CommentDetail{
			Comment:   comment,
			Reactions: commentReactions,
		}
	}
	postDetail := &dto.PostDetail{
		Post:      post,
		Comments:  commentDetails,
		Reactions: reactions,
	}
	return postDetail, nil
}

func (s *PostService) UpdatePost(id, authorID int64, title, content string) error {
	// 게시글 수정 로직 구현
	post, err := s.repository.PostRepository.SelectPostByID(id) // post 존재 여부 확인
	if post == nil || err != nil {
		return customError.ErrInternalServerError
	}
	if post.AuthorID != authorID {
		return customError.ErrInternalServerError
	}
	err = s.repository.PostRepository.UpdatePost(id, title, content)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}

func (s *PostService) DeletePost(id, authorID int64) error {
	// 게시글 삭제 로직 구현
	post, err := s.repository.PostRepository.SelectPostByID(id) // post 존재 여부 확인
	if post == nil || err != nil {
		return customError.ErrInternalServerError
	}
	if post.AuthorID != authorID {
		return customError.ErrInternalServerError
	}
	err = s.repository.PostRepository.DeletePost(id)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}
