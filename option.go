package cron

import (
	"log/slog"
)

type Option func(*cronOptions)

type cronOptions struct {
	logger       *slog.Logger
	withSeconds  bool
	withRecover  bool
	withSkip     bool
	withDelay    bool
	showFuncName bool
	logLevel     slog.Level
}

func WithLogLevel(level slog.Level) Option {
	return func(o *cronOptions) {
		o.logLevel = level
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(o *cronOptions) {
		o.logger = logger
	}
}

func WithSeconds() Option {
	return func(o *cronOptions) {
		o.withSeconds = true
	}
}

func WithRecover() Option {
	return func(o *cronOptions) {
		o.withRecover = true
	}
}

func WithSkippingNewJob() Option {
	return func(o *cronOptions) {
		o.withSkip = true
	}
}

func WithDelayNewJob() Option {
	return func(o *cronOptions) {
		o.withDelay = true
	}
}

func WithFuncName() Option {
	return func(o *cronOptions) {
		o.showFuncName = true
	}
}
