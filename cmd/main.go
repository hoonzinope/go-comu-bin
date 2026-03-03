package main

import (
	"fmt"
	"log"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	"github.com/hoonzinope/go-comu-bin/internal/config"
	"github.com/hoonzinope/go-comu-bin/internal/delivery"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
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

	useCases := application.UseCase{
		UserUseCase:     service.NewUserService(repository),
		BoardUseCase:    service.NewBoardService(repository),
		PostUseCase:     service.NewPostService(repository),
		CommentUseCase:  service.NewCommentService(repository),
		ReactionUseCase: service.NewReactionService(repository),
	}

	server := delivery.NewHTTPServer(port(cfg), jwtSecret(cfg), useCases)
	log.Printf("server started on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func seedAdmin(repository application.Repository) {
	admin := &entity.User{}
	admin.NewAdmin("admin", "admin")
	_, _ = repository.UserRepository.Save(admin)
}

func port(cfg *config.Config) string {
	return fmt.Sprintf(":%d", cfg.Delivery.HTTP.Port)
}

func jwtSecret(cfg *config.Config) string {
	return cfg.Delivery.HTTP.Auth.Secret
}
