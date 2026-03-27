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
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
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
	cacheRistretto "github.com/hoonzinope/go-comu-bin/internal/infrastructure/cache/ristretto"
	eventOutbox "github.com/hoonzinope/go-comu-bin/internal/infrastructure/event/outbox"
	jobrunner "github.com/hoonzinope/go-comu-bin/internal/infrastructure/job/inprocess"
	noopmail "github.com/hoonzinope/go-comu-bin/internal/infrastructure/mail/noop"
	smtpmail "github.com/hoonzinope/go-comu-bin/internal/infrastructure/mail/smtp"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/inmemory"
	sqlitepersist "github.com/hoonzinope/go-comu-bin/internal/infrastructure/persistence/sqlite"
	rateLimitInMemory "github.com/hoonzinope/go-comu-bin/internal/infrastructure/ratelimit/inmemory"
	"github.com/hoonzinope/go-comu-bin/internal/infrastructure/storage/localfs"
	objectstorage "github.com/hoonzinope/go-comu-bin/internal/infrastructure/storage/object"
)

func main() {
	bootstrapLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(bootstrapLogger)
	exitCode := 0
	var logCloser io.Closer
	defer func() {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}()
	defer func() {
		if logCloser != nil {
			if closeErr := logCloser.Close(); closeErr != nil {
				bootstrapLogger.Warn("failed to close log file", "error", closeErr)
			}
		}
	}()
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error("application panicked", "panic", recovered, "stack", string(debug.Stack()))
			exitCode = 1
		}
	}()

	// load config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		exitCode = 1
		return
	}
	appLogger, closer, err := newAppLogger(os.Stdout, cfg)
	if err != nil {
		slog.Error("failed to initialize logger", "error", err)
		exitCode = 1
		return
	}
	logCloser = closer
	slog.SetDefault(appLogger)
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	authDB, err := sqlitepersist.Open(appCtx, sqlitepersist.Options{Path: cfg.Database.Path})
	if err != nil {
		slog.Error("failed to initialize sqlite auth database", "error", err)
		exitCode = 1
		return
	}
	defer func() {
		if closeErr := authDB.Close(); closeErr != nil {
			slog.Warn("failed to close sqlite auth database", "error", closeErr)
		}
	}()

	userRepository := sqlitepersist.NewUserRepository(authDB)
	boardRepository := sqlitepersist.NewBoardRepository(authDB)
	tagRepository := sqlitepersist.NewTagRepository(authDB)
	postTagRepository := sqlitepersist.NewPostTagRepository(authDB)
	postRepository := sqlitepersist.NewPostRepository(authDB)
	postSearchRepository := sqlitepersist.NewPostSearchRepository(authDB)
	postRankingRepository := inmemory.NewPostRankingRepository()
	commentRepository := sqlitepersist.NewCommentRepository(authDB)
	reactionRepository := sqlitepersist.NewReactionRepository(authDB)
	attachmentRepository := sqlitepersist.NewAttachmentRepository(authDB)
	reportRepository := sqlitepersist.NewReportRepository(authDB)
	notificationRepository := sqlitepersist.NewNotificationRepository(authDB)
	outboxRepository := sqlitepersist.NewOutboxRepository(
		authDB,
		sqlitepersist.WithProcessingTimeout(time.Duration(cfg.Event.Outbox.ProcessingLeaseMillis)*time.Millisecond),
	)
	fileStorage, err := newFileStorage(cfg)
	if err != nil {
		slog.Error("failed to initialize file storage", "error", err)
		exitCode = 1
		return
	}

	if err := ensureBootstrapAdmin(cfg, userRepository); err != nil {
		slog.Error("failed to ensure bootstrap admin user", "error", err)
		exitCode = 1
		return
	}
	cache, err := cacheRistretto.NewCache(cacheRistretto.Config{
		NumCounters: cfg.Cache.NumCounters,
		MaxCost:     cfg.Cache.MaxCost,
		BufferItems: cfg.Cache.BufferItems,
		Metrics:     cfg.Cache.Metrics,
	})
	if err != nil {
		slog.Error("failed to initialize cache", "error", err)
		exitCode = 1
		return
	}
	defer cache.Close()
	rateLimiter := rateLimitInMemory.NewInMemoryRateLimiter()
	authorizationPolicy := policy.NewRoleAuthorizationPolicy()
	passwordHasher := auth.NewBcryptPasswordHasher(0)
	mailers := newMailSenders(cfg)
	emailVerificationRepository := sqlitepersist.NewEmailVerificationTokenRepository(authDB)
	passwordResetRepository := sqlitepersist.NewPasswordResetTokenRepository(authDB)
	unitOfWork := sqlitepersist.NewUnitOfWork(authDB, boardRepository, postRepository, tagRepository, postTagRepository, commentRepository, reactionRepository, attachmentRepository, reportRepository, notificationRepository, emailVerificationRepository, passwordResetRepository, outboxRepository)
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
	postSearchIndexHandler := appevent.NewPostSearchIndexHandler(postSearchRepository)
	postRankingHandler := appevent.NewPostRankingHandler(postRankingRepository)
	notificationHandler := appevent.NewNotificationHandler(notificationRepository)
	mailDeliveryHandler := appevent.NewMailDeliveryHandler(mailers.EmailVerificationMailSender, mailers.PasswordResetMailSender, emailVerificationRepository, passwordResetRepository)
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
	outboxRelay.Subscribe(appevent.EventNameSignupEmailVerificationRequested, mailDeliveryHandler)
	outboxRelay.Subscribe(appevent.EventNameEmailVerificationResendRequested, mailDeliveryHandler)
	outboxRelay.Subscribe(appevent.EventNamePasswordResetRequested, mailDeliveryHandler)
	if err := postSearchRepository.RebuildAll(appCtx); err != nil {
		slog.Error("failed to build post search index", "error", err)
		exitCode = 1
		return
	}
	outboxRelay.Start(appCtx)
	userUseCase := service.NewUserServiceWithEmailVerification(userRepository, passwordHasher, unitOfWork, emailVerificationRepository, auth.NewEmailVerificationTokenIssuer(), 30*time.Minute)
	boardUseCase := service.NewBoardServiceWithActionDispatcher(userRepository, boardRepository, postRepository, unitOfWork, cache, nil, cachePolicy(cfg), authorizationPolicy, appLogger)
	postUseCase := service.NewPostServiceWithActionDispatcher(userRepository, boardRepository, postRepository, postSearchRepository, postRankingRepository, tagRepository, postTagRepository, attachmentRepository, commentRepository, reactionRepository, unitOfWork, cache, nil, cachePolicy(cfg), authorizationPolicy, appLogger)
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
	emailVerificationCleanupUseCase := service.NewEmailVerificationCleanupService(emailVerificationRepository)
	passwordResetCleanupUseCase := service.NewPasswordResetCleanupService(passwordResetRepository)
	if err := startBackgroundJobs(appCtx, slog.Default(), cfg, attachmentUseCase, guestCleanupUseCase, emailVerificationCleanupUseCase, passwordResetCleanupUseCase); err != nil {
		slog.Error("failed to start background jobs", "error", err)
		exitCode = 1
		return
	}
	sessionUseCase := service.NewSessionService(userUseCase, userUseCase, userRepository, tokenProvider, sessionRepository, appLogger)
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
		30*time.Minute,
		passwordResetRepository,
		passwordResetIssuer,
		30*time.Minute,
		appLogger,
	)
	server := delivery.NewHTTPServer(httpAddr(cfg), delivery.HTTPDependencies{
		SessionUseCase:                         sessionUseCase,
		AdminAuthorizer:                        userUseCase,
		UserUseCase:                            userUseCase,
		AccountUseCase:                         accountUseCase,
		BoardUseCase:                           boardUseCase,
		PostUseCase:                            postUseCase,
		CommentUseCase:                         commentUseCase,
		NotificationUseCase:                    notificationUseCase,
		ReactionUseCase:                        reactionUseCase,
		AttachmentUseCase:                      attachmentUseCase,
		ReportUseCase:                          reportUseCase,
		OutboxAdminUseCase:                     outboxAdminUseCase,
		RateLimiter:                            rateLimiter,
		AttachmentUploadMaxBytes:               cfg.Storage.Attachment.MaxUploadSizeBytes,
		MaxJSONBodyBytes:                       cfg.Delivery.HTTP.MaxJSONBodyBytes,
		DefaultPageLimit:                       cfg.Delivery.HTTP.DefaultPageLimit,
		RateLimitEnabled:                       cfg.Delivery.HTTP.RateLimit.Enabled,
		RateLimitWindowSecond:                  cfg.Delivery.HTTP.RateLimit.WindowSeconds,
		RateLimitReadRequest:                   cfg.Delivery.HTTP.RateLimit.ReadRequests,
		RateLimitWriteRequest:                  cfg.Delivery.HTTP.RateLimit.WriteRequests,
		LoginRateLimitEnabled:                  cfg.Delivery.HTTP.Auth.LoginRateLimit.Enabled,
		LoginRateLimitWindowSecond:             cfg.Delivery.HTTP.Auth.LoginRateLimit.WindowSeconds,
		LoginRateLimitMaxRequests:              cfg.Delivery.HTTP.Auth.LoginRateLimit.MaxRequests,
		GuestUpgradeRateLimitEnabled:           cfg.Delivery.HTTP.Auth.GuestUpgradeRateLimit.Enabled,
		GuestUpgradeRateLimitWindowSecond:      cfg.Delivery.HTTP.Auth.GuestUpgradeRateLimit.WindowSeconds,
		GuestUpgradeRateLimitMaxRequests:       cfg.Delivery.HTTP.Auth.GuestUpgradeRateLimit.MaxRequests,
		EmailVerificationRateLimitEnabled:      cfg.Delivery.HTTP.Auth.EmailVerificationRequestRateLimit.Enabled,
		EmailVerificationRateLimitWindowSecond: cfg.Delivery.HTTP.Auth.EmailVerificationRequestRateLimit.WindowSeconds,
		EmailVerificationRateLimitMaxRequests:  cfg.Delivery.HTTP.Auth.EmailVerificationRequestRateLimit.MaxRequests,
		PasswordResetRateLimitEnabled:          cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.Enabled,
		PasswordResetRateLimitWindowSecond:     cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.WindowSeconds,
		PasswordResetRateLimitMaxRequests:      cfg.Delivery.HTTP.Auth.PasswordResetRequestRateLimit.MaxRequests,
		Logger:                                 appLogger,
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
		err = gracefulShutdown(server, serverErrCh, outboxRelay, cancel, 5*time.Second, appLogger)
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server stopped with error", "error", err)
		exitCode = 1
		return
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

func startBackgroundJobs(ctx context.Context, logger *slog.Logger, cfg *config.Config, attachmentCleanupUseCase port.AttachmentCleanupUseCase, guestCleanupUseCase port.GuestCleanupUseCase, emailVerificationCleanupUseCase port.EmailVerificationCleanupUseCase, passwordResetCleanupUseCase port.PasswordResetCleanupUseCase) error {
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
	if cfg.Jobs.EmailVerificationCleanup.Enabled {
		interval := time.Duration(cfg.Jobs.EmailVerificationCleanup.IntervalSeconds) * time.Second
		gracePeriod := time.Duration(cfg.Jobs.EmailVerificationCleanup.GracePeriodSeconds) * time.Second
		batchSize := cfg.Jobs.EmailVerificationCleanup.BatchSize
		if err := runner.Register(jobrunner.Job{
			Name:     "email-verification-cleanup",
			Interval: interval,
			Run: func(ctx context.Context) error {
				if emailVerificationCleanupUseCase == nil {
					return nil
				}
				_, err := emailVerificationCleanupUseCase.CleanupEmailVerificationTokens(ctx, time.Now(), gracePeriod, batchSize)
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
