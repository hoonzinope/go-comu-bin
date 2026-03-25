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
	"os/signal"
	"strings"
	"syscall"
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
	noopmail "github.com/hoonzinope/go-comu-bin/internal/infrastructure/mail/noop"
	smtpmail "github.com/hoonzinope/go-comu-bin/internal/infrastructure/mail/smtp"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
	rateLimitInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/ratelimit/inmemory"
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
	postSearchStore := inmemory.NewPostSearchStore(postRepository, tagRepository, postTagRepository)
	postRankingRepository := inmemory.NewPostRankingRepository()
	commentRepository := inmemory.NewCommentRepository()
	reactionRepository := inmemory.NewReactionRepository()
	attachmentRepository := inmemory.NewAttachmentRepository()
	reportRepository := inmemory.NewReportRepository()
	notificationRepository := inmemory.NewNotificationRepository()
	emailVerificationRepository := inmemory.NewEmailVerificationTokenRepository()
	passwordResetRepository := inmemory.NewPasswordResetTokenRepository()
	outboxRepository := inmemory.NewOutboxRepository(
		inmemory.WithProcessingTimeout(time.Duration(cfg.Event.Outbox.ProcessingLeaseMillis) * time.Millisecond),
	)
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
	rateLimiter := rateLimitInMemory.NewInMemoryRateLimiter()
	authorizationPolicy := policy.NewRoleAuthorizationPolicy()
	passwordHasher := auth.NewBcryptPasswordHasher(0)
	appLogger := logger
	unitOfWork := inmemory.NewUnitOfWork(userRepository, boardRepository, postRepository, tagRepository, postTagRepository, commentRepository, reactionRepository, attachmentRepository, reportRepository, notificationRepository, emailVerificationRepository, passwordResetRepository, outboxRepository)
	eventSerializer := appevent.NewJSONEventSerializer()
	outboxRelay := eventOutbox.NewRelay(
		outboxRepository,
		eventSerializer,
		appLogger,
		eventOutbox.RelayConfig{
			WorkerCount:     cfg.Event.Outbox.WorkerCount,
			BatchSize:       cfg.Event.Outbox.BatchSize,
			PollInterval:    time.Duration(cfg.Event.Outbox.PollIntervalMillis) * time.Millisecond,
			MaxAttempts:     cfg.Event.Outbox.MaxAttempts,
			BaseBackoff:     time.Duration(cfg.Event.Outbox.BaseBackoffMillis) * time.Millisecond,
			ProcessingLease: time.Duration(cfg.Event.Outbox.ProcessingLeaseMillis) * time.Millisecond,
			LeaseRefresh:    time.Duration(cfg.Event.Outbox.LeaseRefreshMillis) * time.Millisecond,
		},
	)
	cacheInvalidationHandler := appevent.NewCacheInvalidationHandler(cache, appLogger)
	postSearchIndexHandler := appevent.NewPostSearchIndexHandler(postSearchStore)
	postRankingHandler := appevent.NewPostRankingHandler(postRankingRepository)
	notificationHandler := appevent.NewNotificationHandler(notificationRepository)
	outboxRelay.Subscribe(appevent.EventNameBoardChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNamePostChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNamePostChanged, postSearchIndexHandler)
	outboxRelay.Subscribe(appevent.EventNamePostChanged, postRankingHandler)
	outboxRelay.Subscribe(appevent.EventNameCommentChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameCommentChanged, postRankingHandler)
	outboxRelay.Subscribe(appevent.EventNameReactionChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameReactionChanged, postRankingHandler)
	outboxRelay.Subscribe(appevent.EventNameAttachmentChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameReportChanged, cacheInvalidationHandler)
	outboxRelay.Subscribe(appevent.EventNameNotificationTriggered, notificationHandler)
	if err := postSearchStore.RebuildAll(appCtx); err != nil {
		slog.Error("failed to build post search index", "error", err)
		os.Exit(1)
	}
	outboxRelay.Start(appCtx)

	mailers := newMailSenders(cfg)
	userUseCase := service.NewUserServiceWithEmailVerification(userRepository, passwordHasher, unitOfWork, emailVerificationRepository, auth.NewEmailVerificationTokenIssuer(), mailers, 30*time.Minute)
	boardUseCase := service.NewBoardServiceWithActionDispatcher(userRepository, boardRepository, postRepository, unitOfWork, cache, nil, cachePolicy(cfg), authorizationPolicy, appLogger)
	postUseCase := service.NewPostServiceWithActionDispatcher(userRepository, boardRepository, postRepository, postSearchStore, postRankingRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy(cfg), authorizationPolicy, appLogger)
	commentUseCase := service.NewCommentServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy(cfg), authorizationPolicy, appLogger)
	notificationUseCase := service.NewNotificationService(userRepository, postRepository, commentRepository, notificationRepository)
	reactionUseCase := service.NewReactionServiceWithActionDispatcher(userRepository, boardRepository, postRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy(cfg), appLogger)
	reportUseCase := service.NewReportServiceWithActionDispatcher(userRepository, postRepository, commentRepository, reportRepository, unitOfWork, nil, authorizationPolicy, appLogger)
	outboxAdminUseCase := service.NewOutboxAdminService(userRepository, outboxRepository, authorizationPolicy, appLogger)
	attachmentUseCase := service.NewAttachmentServiceWithActionDispatcher(
		userRepository,
		boardRepository,
		postRepository,
		attachmentRepository,
		unitOfWork,
		fileStorage,
		cache,
		nil,
		cfg.Storage.Attachment.MaxUploadSizeBytes,
		service.ImageOptimizationConfig{
			Enabled:     cfg.Storage.Attachment.ImageOptimization.Enabled,
			JPEGQuality: cfg.Storage.Attachment.ImageOptimization.JPEGQuality,
		},
		authorizationPolicy,
		appLogger,
	)
	tokenProvider := auth.NewJwtTokenProvider(jwtSecret(cfg))
	passwordResetIssuer := auth.NewPasswordResetTokenIssuer()
	sessionRepository := auth.NewCacheSessionRepository(cache)
	guestCleanupUseCase := service.NewGuestCleanupService(userRepository, postRepository, commentRepository, reactionRepository, reportRepository, sessionRepository, unitOfWork)
	passwordResetCleanupUseCase := service.NewPasswordResetCleanupService(passwordResetRepository)
	if err := startBackgroundJobs(appCtx, slog.Default(), cfg, attachmentUseCase, guestCleanupUseCase, passwordResetCleanupUseCase); err != nil {
		slog.Error("failed to start background jobs", "error", err)
		os.Exit(1)
	}
	sessionUseCase := service.NewSessionService(userUseCase, userUseCase, userRepository, tokenProvider, sessionRepository)
	accountUseCase := service.NewAccountServiceWithGuestUpgrade(
		userUseCase,
		sessionUseCase,
		userRepository,
		unitOfWork,
		passwordHasher,
		tokenProvider,
		sessionRepository,
		emailVerificationRepository,
		auth.NewEmailVerificationTokenIssuer(),
		mailers,
		30*time.Minute,
		passwordResetRepository,
		passwordResetIssuer,
		mailers,
		30*time.Minute,
		appLogger,
	)
	server := delivery.NewHTTPServer(httpAddr(cfg), delivery.HTTPDependencies{
		SessionUseCase:                     sessionUseCase,
		AdminAuthorizer:                    userUseCase,
		UserUseCase:                        userUseCase,
		AccountUseCase:                     accountUseCase,
		BoardUseCase:                       boardUseCase,
		PostUseCase:                        postUseCase,
		CommentUseCase:                     commentUseCase,
		NotificationUseCase:                notificationUseCase,
		ReactionUseCase:                    reactionUseCase,
		AttachmentUseCase:                  attachmentUseCase,
		ReportUseCase:                      reportUseCase,
		OutboxAdminUseCase:                 outboxAdminUseCase,
		RateLimiter:                        rateLimiter,
		AttachmentUploadMaxBytes:           cfg.Storage.Attachment.MaxUploadSizeBytes,
		MaxJSONBodyBytes:                   cfg.Delivery.HTTP.MaxJSONBodyBytes,
		DefaultPageLimit:                   cfg.Delivery.HTTP.DefaultPageLimit,
		RateLimitEnabled:                   cfg.Delivery.HTTP.RateLimit.Enabled,
		RateLimitWindowSecond:              cfg.Delivery.HTTP.RateLimit.WindowSeconds,
		RateLimitReadRequest:               cfg.Delivery.HTTP.RateLimit.ReadRequests,
		RateLimitWriteRequest:              cfg.Delivery.HTTP.RateLimit.WriteRequests,
		PasswordResetRateLimitEnabled:      cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.Enabled,
		PasswordResetRateLimitWindowSecond: cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.WindowSeconds,
		PasswordResetRateLimitMaxRequests:  cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.MaxRequests,
		Logger:                             appLogger,
	})
	slog.Info("server started", "addr", server.Addr)
	signalCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.ListenAndServe()
	}()
	select {
	case err = <-serverErrCh:
		cancel()
		outboxRelay.Wait()
	case <-signalCtx.Done():
		slog.Info("shutdown signal received")
		err = gracefulShutdown(server, serverErrCh, outboxRelay, cancel, 5*time.Second, logger)
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

type httpShutdowner interface {
	Shutdown(ctx context.Context) error
	Close() error
}

type relayWaiter interface {
	Wait()
}

func gracefulShutdown(server httpShutdowner, serverErrCh <-chan error, relay relayWaiter, cancel context.CancelFunc, timeout time.Duration, logger *slog.Logger) error {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	shutdownCtx, shutdownCancel := context.WithDeadline(context.Background(), deadline)
	defer shutdownCancel()
	if server != nil {
		if err := server.Shutdown(shutdownCtx); err != nil && logger != nil {
			logger.Warn("http server shutdown failed", "error", err)
		}
	}
	if cancel != nil {
		cancel()
	}
	waitForRelay(relay, time.Until(deadline), logger)
	if serverErrCh == nil {
		return nil
	}
	if err, ok := awaitServerErr(serverErrCh, time.Until(deadline)); ok {
		return err
	}
	if logger != nil {
		logger.Warn("server did not stop in time, forcing close")
	}
	if server != nil {
		if err := server.Close(); err != nil && logger != nil {
			logger.Warn("http server force close failed", "error", err)
		}
	}
	forceWait := 500 * time.Millisecond
	if remaining := time.Until(deadline); remaining <= 0 {
		forceWait = 0
	} else if remaining < forceWait {
		forceWait = remaining
	}
	if err, ok := awaitServerErr(serverErrCh, forceWait); ok {
		return err
	}
	return fmt.Errorf("graceful shutdown timed out waiting for server stop")
}

func waitForRelay(relay relayWaiter, timeout time.Duration, logger *slog.Logger) {
	if relay == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		relay.Wait()
		close(done)
	}()
	if timeout <= 0 {
		select {
		case <-done:
		default:
			if logger != nil {
				logger.Warn("relay wait timed out during shutdown")
			}
		}
		return
	}
	select {
	case <-done:
	case <-time.After(timeout):
		if logger != nil {
			logger.Warn("relay wait timed out during shutdown")
		}
	}
}

func awaitServerErr(serverErrCh <-chan error, timeout time.Duration) (error, bool) {
	if serverErrCh == nil {
		return nil, false
	}
	if timeout <= 0 {
		select {
		case err := <-serverErrCh:
			return err, true
		default:
			return nil, false
		}
	}
	select {
	case err := <-serverErrCh:
		return err, true
	case <-time.After(timeout):
		return nil, false
	}
}

func ensureBootstrapAdmin(cfg *config.Config, userRepository port.UserRepository) error {
	if cfg == nil || !cfg.Admin.Bootstrap.Enabled {
		return nil
	}
	username := strings.TrimSpace(cfg.Admin.Bootstrap.Username)
	password := cfg.Admin.Bootstrap.Password
	existingUser, err := userRepository.SelectUserByUsername(context.Background(), username)
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
	_, err = userRepository.Save(context.Background(), admin)
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

type mailSenders struct {
	port.PasswordResetMailSender
	port.EmailVerificationMailSender
}

func newMailSenders(cfg *config.Config) mailSenders {
	if cfg != nil && cfg.Delivery.Mail.Enabled {
		sender := smtpmail.NewSender(*cfg)
		return mailSenders{
			PasswordResetMailSender:     sender,
			EmailVerificationMailSender: sender,
		}
	}
	return mailSenders{
		PasswordResetMailSender:     noopmail.NewPasswordResetMailSender(),
		EmailVerificationMailSender: noopmail.NewEmailVerificationMailSender(),
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

func startBackgroundJobs(ctx context.Context, logger *slog.Logger, cfg *config.Config, attachmentCleanupUseCase port.AttachmentCleanupUseCase, guestCleanupUseCase port.GuestCleanupUseCase, passwordResetCleanupUseCase port.PasswordResetCleanupUseCase) error {
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
	if cfg.Jobs.GuestCleanup.Enabled {
		interval := time.Duration(cfg.Jobs.GuestCleanup.IntervalSeconds) * time.Second
		pendingGrace := time.Duration(cfg.Jobs.GuestCleanup.PendingGracePeriodSeconds) * time.Second
		activeUnusedGrace := time.Duration(cfg.Jobs.GuestCleanup.ActiveUnusedGracePeriodSeconds) * time.Second
		batchSize := cfg.Jobs.GuestCleanup.BatchSize
		if err := runner.Register(jobrunner.Job{
			Name:     "guest-cleanup",
			Interval: interval,
			Run: func(ctx context.Context) error {
				if guestCleanupUseCase == nil {
					return nil
				}
				_, err := guestCleanupUseCase.CleanupGuests(ctx, time.Now(), pendingGrace, activeUnusedGrace, batchSize)
				return err
			},
		}); err != nil {
			return err
		}
	}
	if cfg.Jobs.PasswordResetCleanup.Enabled {
		interval := time.Duration(cfg.Jobs.PasswordResetCleanup.IntervalSeconds) * time.Second
		gracePeriod := time.Duration(cfg.Jobs.PasswordResetCleanup.GracePeriodSeconds) * time.Second
		batchSize := cfg.Jobs.PasswordResetCleanup.BatchSize
		if err := runner.Register(jobrunner.Job{
			Name:     "password-reset-cleanup",
			Interval: interval,
			Run: func(ctx context.Context) error {
				if passwordResetCleanupUseCase == nil {
					return nil
				}
				_, err := passwordResetCleanupUseCase.CleanupPasswordResetTokens(ctx, time.Now(), gracePeriod, batchSize)
				return err
			},
		}); err != nil {
			return err
		}
	}
	runner.Start(ctx)
	return nil
}
