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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/hoonzinope/go-comu-bin/docs/swagger"
	appcache "github.com/hoonzinope/go-comu-bin/internal/application/cache"
	appevent "github.com/hoonzinope/go-comu-bin/internal/application/event"
	"github.com/hoonzinope/go-comu-bin/internal/application/policy"
	"github.com/hoonzinope/go-comu-bin/internal/application/port"
	"github.com/hoonzinope/go-comu-bin/internal/application/service"
	"github.com/hoonzinope/go-comu-bin/internal/config"
	"github.com/hoonzinope/go-comu-bin/internal/delivery"
	"github.com/hoonzinope/go-comu-bin/internal/domain/entity"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/auth"
	cacheInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/inmemory"
	eventOutbox "github.com/hoonzinope/go-comu-bin/internal/infrastructure/event/outbox"
	jobrunner "github.com/hoonzinope/go-comu-bin/internal/infrastructure/job/inprocess"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/logging"
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
	outboxRepository := inmemory.NewOutboxRepository()
	fileStorage, err := newFileStorage(cfg)
	if err != nil {
		slog.Error("failed to initialize file storage", "error", err)
		os.Exit(1)
	}

	if err := ensureBootstrapAdmin(cfg, userRepository); err != nil {
		slog.Error("failed to ensure bootstrap admin user", "error", err)
		os.Exit(1)
	}
	cache := cacheInMemory.NewInMemoryCache()
	authorizationPolicy := policy.NewRoleAuthorizationPolicy()
	passwordHasher := auth.NewBcryptPasswordHasher(0)
	appLogger := logging.NewSlogLogger(logger)
	unitOfWork := inmemory.NewUnitOfWork(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, commentRepository, reactionRepository, attachmentRepository, outboxRepository)
	eventSerializer := appevent.NewJSONEventSerializer()
	outboxRelay := eventOutbox.NewRelay(
		outboxRepository,
		eventSerializer,
		appLogger,
		eventOutbox.RelayConfig{
			WorkerCount:  cfg.Event.Outbox.WorkerCount,
			BatchSize:    cfg.Event.Outbox.BatchSize,
			PollInterval: time.Duration(cfg.Event.Outbox.PollIntervalMillis) * time.Millisecond,
			MaxAttempts:  cfg.Event.Outbox.MaxAttempts,
			BaseBackoff:  time.Duration(cfg.Event.Outbox.BaseBackoffMillis) * time.Millisecond,
		},
	)
	outboxPublisher := eventOutbox.NewPublisher(outboxRepository, eventSerializer, appLogger)
	cacheInvalidationHandler := appevent.NewCacheInvalidationHandler(cache, appLogger)
	outboxRelay.Subscribe(appevent.EventNameBoardChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNamePostChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameCommentChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameReactionChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameAttachmentChanged, cacheInvalidationHandler)
	outboxRelay.Start(appCtx)

	userUseCase := service.NewUserService(userRepository, passwordHasher, unitOfWork)
	boardUseCase := service.NewBoardServiceWithPublisher(userRepository, boardRepository, postRepository, unitOfWork, cache, outboxPublisher, cachePolicy(cfg), authorizationPolicy, appLogger)
	postUseCase := service.NewPostServiceWithPublisher(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, outboxPublisher, cachePolicy(cfg), authorizationPolicy, appLogger)
	commentUseCase := service.NewCommentServiceWithPublisher(userRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, outboxPublisher, cachePolicy(cfg), authorizationPolicy, appLogger)
	reactionUseCase := service.NewReactionServiceWithPublisher(userRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, outboxPublisher, cachePolicy(cfg), appLogger)
	attachmentUseCase := service.NewAttachmentServiceWithPublisher(
		userRepository,
		postRepository,
		attachmentRepository,
		unitOfWork,
		fileStorage,
		cache,
		outboxPublisher,
		cfg.Storage.Attachment.MaxUploadSizeBytes,
		service.ImageOptimizationConfig{
			Enabled:     cfg.Storage.Attachment.ImageOptimization.Enabled,
			JPEGQuality: cfg.Storage.Attachment.ImageOptimization.JPEGQuality,
		},
		authorizationPolicy,
		appLogger,
	)
	if err := startBackgroundJobs(appCtx, slog.Default(), cfg, attachmentUseCase); err != nil {
		slog.Error("failed to start background jobs", "error", err)
		os.Exit(1)
	}

	tokenProvider := auth.NewJwtTokenProvider(jwtSecret(cfg))
	sessionRepository := auth.NewCacheSessionRepository(cache)
	sessionUseCase := service.NewSessionService(userUseCase, userRepository, tokenProvider, sessionRepository)
	accountUseCase := service.NewAccountService(userUseCase, sessionUseCase, appLogger)
	server := delivery.NewHTTPServer(httpAddr(cfg), delivery.HTTPDependencies{
		SessionUseCase:           sessionUseCase,
		UserUseCase:              userUseCase,
		AccountUseCase:           accountUseCase,
		BoardUseCase:             boardUseCase,
		PostUseCase:              postUseCase,
		CommentUseCase:           commentUseCase,
		ReactionUseCase:          reactionUseCase,
		AttachmentUseCase:        attachmentUseCase,
		AttachmentUploadMaxBytes: cfg.Storage.Attachment.MaxUploadSizeBytes,
		MaxJSONBodyBytes:         cfg.Delivery.HTTP.MaxJSONBodyBytes,
		Logger:                   appLogger,
	})
	slog.Info("server started", "addr", server.Addr)
	err = server.ListenAndServe()
	cancel()
	outboxRelay.Wait()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func ensureBootstrapAdmin(cfg *config.Config, userRepository port.UserRepository) error {
	if cfg == nil || !cfg.Admin.Bootstrap.Enabled {
		return nil
	}
	username := strings.TrimSpace(cfg.Admin.Bootstrap.Username)
	password := cfg.Admin.Bootstrap.Password
	existingUser, err := userRepository.SelectUserByUsername(username)
	if err != nil {
		return err
	}
	if existingUser != nil {
		return nil
	}

	passwordHasher := auth.NewBcryptPasswordHasher(0)
	hashedPassword, err := passwordHasher.Hash(password)
	if err != nil {
		return err
	}
	admin := entity.NewAdmin(username, hashedPassword)
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
