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
	"github.com/hoonzinope/go-comu-bin/internal/application"
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
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

	repository := application.Repository{
		UserRepository:     inmemory.NewUserRepository(),
		BoardRepository:    inmemory.NewBoardRepository(),
		PostRepository:     inmemory.NewPostRepository(),
		CommentRepository:  inmemory.NewCommentRepository(),
		ReactionRepository: inmemory.NewReactionRepository(),
	}

	seedAdmin(repository)
	cache := cacheInMemory.NewInMemoryCache()

	useCases := application.UseCase{
		UserUseCase:     service.NewUserService(repository),
		BoardUseCase:    service.NewBoardService(repository, cache, cachePolicy(cfg)),
		PostUseCase:     service.NewPostService(repository, cache, cachePolicy(cfg)),
		CommentUseCase:  service.NewCommentService(repository, cache, cachePolicy(cfg)),
		ReactionUseCase: service.NewReactionService(repository, cache, cachePolicy(cfg)),
	}

	authUseCase := auth.NewJwtTokenProvider(jwtSecret(cfg))
	server := delivery.NewHTTPServer(port(cfg), authUseCase, cache, useCases)
	log.Printf("server started on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func seedAdmin(repository application.Repository) {
	admin := entity.NewAdmin("admin", "admin")
	_, _ = repository.UserRepository.Save(admin)
}

func port(cfg *config.Config) string {
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
