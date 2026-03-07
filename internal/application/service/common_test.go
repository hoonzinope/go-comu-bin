package service

import (
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	noopCache "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/noop"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
)

type testRepositories struct {
	user     port.UserRepository
	board    port.BoardRepository
	post     port.PostRepository
	comment  port.CommentRepository
	reaction port.ReactionRepository
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

func newTestCache() port.Cache {
	return noopCache.NewNoopCache()
}

func newTestCachePolicy() appcache.Policy {
	return appcache.Policy{
		ListTTLSeconds:   30,
		DetailTTLSeconds: 30,
	}
}

func newTestAuthorizationPolicy() policy.AuthorizationPolicy {
	return policy.NewRoleAuthorizationPolicy()
}

func seedUser(userRepository port.UserRepository, name, password, role string) int64 {
	var user *entity.User
	if role == "admin" {
		user = entity.NewAdmin(name, password)
	} else {
		user = entity.NewUser(name, password)
	}
	id, _ := userRepository.Save(user)
	return id
}

func seedBoard(boardRepository port.BoardRepository, name, description string) int64 {
	board := entity.NewBoard(name, description)
	id, _ := boardRepository.Save(board)
	return id
}

func seedPost(postRepository port.PostRepository, authorID, boardID int64, title, content string) int64 {
	post := entity.NewPost(title, content, authorID, boardID)
	id, _ := postRepository.Save(post)
	return id
}

func seedComment(commentRepository port.CommentRepository, authorID, postID int64, content string) int64 {
	comment := entity.NewComment(content, authorID, postID, nil)
	id, _ := commentRepository.Save(comment)
	return id
}
