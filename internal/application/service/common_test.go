package service

import (
	"github.com/hoonzinope/go-comu-bin/internal/application"
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	noopCache "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/noop"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
)

type testRepositories struct {
	user     application.UserRepository
	board    application.BoardRepository
	post     application.PostRepository
	comment  application.CommentRepository
	reaction application.ReactionRepository
}

func newTestRepositories() testRepositories {
	return testRepositories{
		user:     inmemory.NewUserRepository(),
		board:    inmemory.NewBoardRepository(),
		post:     inmemory.NewPostRepository(),
		comment:  inmemory.NewCommentRepository(),
		reaction: inmemory.NewReactionRepository(),
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

func seedUser(userRepository application.UserRepository, name, password, role string) int64 {
	var user *entity.User
	if role == "admin" {
		user = entity.NewAdmin(name, password)
	} else {
		user = entity.NewUser(name, password)
	}
	id, _ := userRepository.Save(user)
	return id
}

func seedBoard(boardRepository application.BoardRepository, name, description string) int64 {
	board := entity.NewBoard(name, description)
	id, _ := boardRepository.Save(board)
	return id
}

func seedPost(postRepository application.PostRepository, authorID, boardID int64, title, content string) int64 {
	post := entity.NewPost(title, content, authorID, boardID)
	id, _ := postRepository.Save(post)
	return id
}

func seedComment(commentRepository application.CommentRepository, authorID, postID int64, content string) int64 {
	comment := entity.NewComment(content, authorID, postID, nil)
	id, _ := commentRepository.Save(comment)
	return id
}
