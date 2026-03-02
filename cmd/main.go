package main

import (
	"log"

	"github.com/hoonzinope/go-comu-bin/internal/application"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	"github.com/hoonzinope/go-comu-bin/internal/delivery"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
)

func main() {
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

	server := delivery.NewHTTPServer(":18577", useCases)
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
