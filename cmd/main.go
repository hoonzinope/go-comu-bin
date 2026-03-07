// Package main go-comu-bin server entrypoint
//
// @title go-comu-bin API
// @version 1.0
// @description REST API for go-comu-bin.
// @BasePath /api/v1
// @schemes http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer <token>
package main

import (
	"fmt"
	"log"

	_ "github.com/hoonzinope/go-comu-bin/docs/swagger"
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	"github.com/hoonzinope/go-comu-bin/internal/config"
	"github.com/hoonzinope/go-comu-bin/internal/delivery"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
)

func main() {
	// load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	userRepository := inmemory.NewUserRepository()
	boardRepository := inmemory.NewBoardRepository()
	postRepository := inmemory.NewPostRepository()
	commentRepository := inmemory.NewCommentRepository()
	reactionRepository := inmemory.NewReactionRepository()

	seedAdmin(userRepository)
	cache := cacheInMemory.NewInMemoryCache()

	userUseCase := service.NewUserService(userRepository)
	boardUseCase := service.NewBoardService(userRepository, boardRepository, cache, cachePolicy(cfg))
	postUseCase := service.NewPostService(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, cache, cachePolicy(cfg))
	commentUseCase := service.NewCommentService(userRepository, postRepository, commentRepository, cache, cachePolicy(cfg))
	reactionUseCase := service.NewReactionService(userRepository, postRepository, commentRepository, reactionRepository, cache, cachePolicy(cfg))

	tokenProvider := auth.NewJwtTokenProvider(jwtSecret(cfg))
	sessionUseCase := service.NewSessionService(userUseCase, tokenProvider, cache)
	server := delivery.NewHTTPServer(httpAddr(cfg), delivery.HTTPDependencies{
		SessionUseCase:  sessionUseCase,
		UserUseCase:     userUseCase,
		BoardUseCase:    boardUseCase,
		PostUseCase:     postUseCase,
		CommentUseCase:  commentUseCase,
		ReactionUseCase: reactionUseCase,
	})
	log.Printf("server started on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func seedAdmin(userRepository port.UserRepository) {
	admin := entity.NewAdmin("admin", "admin")
	_, _ = userRepository.Save(admin)
}

func httpAddr(cfg *config.Config) string {
	return fmt.Sprintf(":%d", cfg.Delivery.HTTP.Port)
}

func jwtSecret(cfg *config.Config) string {
	return cfg.Delivery.HTTP.Auth.Secret
}

func cachePolicy(cfg *config.Config) appcache.Policy {
	return appcache.Policy{
		ListTTLSeconds:   cfg.Cache.ListTTLSeconds,
		DetailTTLSeconds: cfg.Cache.DetailTTLSeconds,
	}
}
