package inprocess

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

type Job struct {
	Name     string
	Interval time.Duration
	Run      func(context.Context) error
}

type ticker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct {
	ticker *time.Ticker
}

func (t *realTicker) C() <-chan time.Time { return t.ticker.C }

func (t *realTicker) Stop() { t.ticker.Stop() }

type tickerFactory func(time.Duration) ticker

type Runner struct {
	logger        *slog.Logger
	jobs          []Job
	tickerFactory tickerFactory
}

type Option func(*Runner)

func WithTickerFactory(factory func(time.Duration) ticker) Option {
	return func(r *Runner) {
		r.tickerFactory = factory
	}
}

func NewRunner(logger *slog.Logger, opts ...Option) *Runner {
	runner := &Runner{
		logger: logger,
		tickerFactory: func(interval time.Duration) ticker {
			return &realTicker{ticker: time.NewTicker(interval)}
		},
	}
	for _, opt := range opts {
		opt(runner)
	}
	return runner
}

func (r *Runner) Register(job Job) error {
	if job.Name == "" || job.Interval <= 0 || job.Run == nil {
		return fmt.Errorf("invalid job registration")
	}
	r.jobs = append(r.jobs, job)
	return nil
}

func (r *Runner) Start(ctx context.Context) {
	for _, job := range r.jobs {
		go r.runJob(ctx, job)
	}
}

func (r *Runner) runJob(ctx context.Context, job Job) {
	ticker := r.tickerFactory(job.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			func() {
				defer func() {
					if recovered := recover(); recovered != nil && r.logger != nil {
						r.logger.Error(
							"background job panicked",
							"job", job.Name,
							"panic", recovered,
							"stack", string(debug.Stack()),
						)
					}
				}()
				if err := job.Run(ctx); err != nil && r.logger != nil {
					r.logger.Error("background job failed", "job", job.Name, "error", err)
					return
				}
				if r.logger != nil {
					r.logger.Info("background job completed", "job", job.Name)
				}
			}()
		}
	}
}
