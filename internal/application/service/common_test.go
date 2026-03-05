package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	noopCache "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/noop"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
)

func newTestRepository() application.Repository {
	return application.Repository{
		UserRepository:     inmemory.NewUserRepository(),
		BoardRepository:    inmemory.NewBoardRepository(),
		PostRepository:     inmemory.NewPostRepository(),
		CommentRepository:  inmemory.NewCommentRepository(),
		ReactionRepository: inmemory.NewReactionRepository(),
	}
}

func newTestCache() application.Cache {
	return noopCache.NewNoopCache()
}

func newTestCachePolicy() appcache.Policy {
	return appcache.Policy{
		ListTTLSeconds:   30,
		DetailTTLSeconds: 30,
	}
}

func seedUser(repository application.Repository, name, password, role string) int64 {
	var user *entity.User
	if role == "admin" {
		user = entity.NewAdmin(name, password)
	} else {
		user = entity.NewUser(name, password)
	}
	id, _ := repository.UserRepository.Save(user)
	return id
}

func seedBoard(repository application.Repository, name, description string) int64 {
	board := entity.NewBoard(name, description)
	id, _ := repository.BoardRepository.Save(board)
	return id
}

func seedPost(repository application.Repository, authorID, boardID int64, title, content string) int64 {
	post := entity.NewPost(title, content, authorID, boardID)
	id, _ := repository.PostRepository.Save(post)
	return id
}

func seedComment(repository application.Repository, authorID, postID int64, content string) int64 {
	comment := entity.NewComment(content, authorID, postID, nil)
	id, _ := repository.CommentRepository.Save(comment)
	return id
}
