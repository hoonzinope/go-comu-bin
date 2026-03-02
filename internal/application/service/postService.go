package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	customError "github.com/hoonzinope/go-comu-bin/internal/customError"
	"github.com/hoonzinope/go-comu-bin/internal/domain/dto"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
)

var (
	commentDefaultLimit = 10
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
	newPost := &entity.Post{}
	newPost.NewPost(title, content, authorID, boardID)
	postID, err := s.repository.PostRepository.Save(newPost)
	if err != nil {
		return 0, customError.ErrInternalServerError
	}
	return postID, nil
}

func (s *PostService) GetPostsList(boardID int64, limit, offset int) (*dto.PostList, error) {
	// 게시글 목록 조회 로직 구현
	posts, err := s.repository.PostRepository.SelectPosts(boardID, limit, offset)
	if err != nil {
		return nil, customError.ErrInternalServerError
	}

	return &dto.PostList{
		Posts:  posts,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *PostService) GetPostDetail(id int64) (*dto.PostDetail, error) {
	post, err := s.repository.PostRepository.SelectPostByID(id)
	if post == nil || err != nil {
		return nil, customError.ErrInternalServerError
	}
	reactions, err := s.repository.ReactionRepository.GetByTarget(post.ID, "post")
	if err != nil {
		return nil, customError.ErrInternalServerError
	}
	comments, err := s.repository.CommentRepository.SelectComments(post.ID, commentDefaultLimit, 0) // 댓글은 최대 10개까지 조회
	commentDetails := make([]*dto.CommentDetail, len(comments))
	if err != nil {
		return nil, customError.ErrInternalServerError
	}
	for i, comment := range comments {
		commentReactions, err := s.repository.ReactionRepository.GetByTarget(comment.ID, "comment")
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
	post.UpdatePost(title, content)
	err = s.repository.PostRepository.Update(post)
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
	err = s.repository.PostRepository.Delete(post.ID)
	if err != nil {
		return customError.ErrInternalServerError
	}
	return nil
}
