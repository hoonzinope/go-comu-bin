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
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/hoonzinope/go-comu-bin/docs/swagger"
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	"github.com/hoonzinope/go-comu-bin/internal/config"
	"github.com/hoonzinope/go-comu-bin/internal/delivery"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	jobrunner "github.com/hoonzinope/go-comu-bin/internal/infrastructure/job/inprocess"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/storage/localfs"
	objectstorage "github.com/hoonzinope/go-comu-bin/internal/infrastructure/storage/object"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// load config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	userRepository := inmemory.NewUserRepository()
	boardRepository := inmemory.NewBoardRepository()
	tagRepository := inmemory.NewTagRepository()
	postTagRepository := inmemory.NewPostTagRepository()
	postRepository := inmemory.NewPostRepository(tagRepository, postTagRepository)
	commentRepository := inmemory.NewCommentRepository()
	reactionRepository := inmemory.NewReactionRepository()
	attachmentRepository := inmemory.NewAttachmentRepository()
	fileStorage, err := newFileStorage(cfg)
	if err != nil {
		slog.Error("failed to initialize file storage", "error", err)
		os.Exit(1)
	}

	if err := seedAdmin(userRepository); err != nil {
		slog.Error("failed to seed admin user", "error", err)
		os.Exit(1)
	}
	cache := cacheInMemory.NewInMemoryCache()
	authorizationPolicy := policy.NewRoleAuthorizationPolicy()
	passwordHasher := auth.NewBcryptPasswordHasher(0)
	unitOfWork := inmemory.NewUnitOfWork(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, commentRepository, reactionRepository, attachmentRepository)

	userUseCase := service.NewUserService(userRepository, passwordHasher, unitOfWork)
	boardUseCase := service.NewBoardService(userRepository, boardRepository, postRepository, unitOfWork, cache, cachePolicy(cfg), authorizationPolicy)
	postUseCase := service.NewPostService(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, cachePolicy(cfg), authorizationPolicy)
	commentUseCase := service.NewCommentService(userRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, cachePolicy(cfg), authorizationPolicy)
	reactionUseCase := service.NewReactionService(userRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, cachePolicy(cfg))
	attachmentUseCase := service.NewAttachmentServiceWithOptions(
		userRepository,
		postRepository,
		attachmentRepository,
		unitOfWork,
		fileStorage,
		cache,
		cfg.Storage.Attachment.MaxUploadSizeBytes,
		service.ImageOptimizationConfig{
			Enabled:     cfg.Storage.Attachment.ImageOptimization.Enabled,
			JPEGQuality: cfg.Storage.Attachment.ImageOptimization.JPEGQuality,
		},
		authorizationPolicy,
	)
	if err := startBackgroundJobs(appCtx, slog.Default(), cfg, attachmentUseCase); err != nil {
		slog.Error("failed to start background jobs", "error", err)
		os.Exit(1)
	}

	tokenProvider := auth.NewJwtTokenProvider(jwtSecret(cfg))
	sessionRepository := auth.NewCacheSessionRepository(cache)
	sessionUseCase := service.NewSessionService(userUseCase, tokenProvider, sessionRepository)
	accountUseCase := service.NewAccountService(userUseCase, sessionUseCase)
	server := delivery.NewHTTPServer(httpAddr(cfg), delivery.HTTPDependencies{
		SessionUseCase:    sessionUseCase,
		UserUseCase:       userUseCase,
		AccountUseCase:    accountUseCase,
		BoardUseCase:      boardUseCase,
		PostUseCase:       postUseCase,
		CommentUseCase:    commentUseCase,
		ReactionUseCase:   reactionUseCase,
		AttachmentUseCase: attachmentUseCase,
	})
	slog.Info("server started", "addr", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func seedAdmin(userRepository port.UserRepository) error {
	passwordHasher := auth.NewBcryptPasswordHasher(0)
	hashedPassword, err := passwordHasher.Hash("admin")
	if err != nil {
		return err
	}
	admin := entity.NewAdmin("admin", hashedPassword)
	_, err = userRepository.Save(admin)
	return err
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

func newFileStorage(cfg *config.Config) (port.FileStorage, error) {
	switch cfg.Storage.Provider {
	case "local":
		return localfs.NewFileStorage(cfg.Storage.Local.RootDir), nil
	case "object":
		return objectstorage.NewFileStorage(
			cfg.Storage.Object.Endpoint,
			cfg.Storage.Object.Bucket,
			cfg.Storage.Object.AccessKey,
			cfg.Storage.Object.SecretKey,
			cfg.Storage.Object.UseSSL,
		)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Storage.Provider)
	}
}

func startBackgroundJobs(ctx context.Context, logger *slog.Logger, cfg *config.Config, attachmentCleanupUseCase port.AttachmentCleanupUseCase) error {
	if !cfg.Jobs.Enabled {
		return nil
	}
	runner := jobrunner.NewRunner(logger)
	if cfg.Jobs.AttachmentCleanup.Enabled {
		interval := time.Duration(cfg.Jobs.AttachmentCleanup.IntervalSeconds) * time.Second
		gracePeriod := time.Duration(cfg.Jobs.AttachmentCleanup.GracePeriodSeconds) * time.Second
		batchSize := cfg.Jobs.AttachmentCleanup.BatchSize
		if err := runner.Register(jobrunner.Job{
			Name:     "attachment-cleanup",
			Interval: interval,
			Run: func(ctx context.Context) error {
				_, err := attachmentCleanupUseCase.CleanupAttachments(ctx, time.Now(), gracePeriod, batchSize)
				return err
			},
		}); err != nil {
			return err
		}
	}
	runner.Start(ctx)
	return nil
}
