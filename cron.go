package cron

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/evilgooby/slog_key"
	"github.com/robfig/cron/v3"
)

type (
	Cron struct {
		cron         *cron.Cron
		jobs         []Job
		logger       *slog.Logger
		logLevel     slog.Level
		showFuncName bool
	}

	Job struct {
		Config JobConfig
		Fn     func(context.Context) error
	}

	JobConfig struct {
		Spec       []string
		RunOnStart bool
		Allowed    bool
	}
)

func New(options ...Option) (*Cron, error) {
	opts := &cronOptions{
		logger:   slog.With("component", "cron"),
		logLevel: slog.LevelInfo,
	}

	for _, opt := range options {
		opt(opts)
	}

	cLogger := NewLogger(opts.logger)

	var cronOpts []cron.Option
	if opts.withSeconds {
		cronOpts = append(cronOpts, cron.WithSeconds())
	}

	var chain []cron.JobWrapper
	if opts.withSkip {
		chain = append(chain, cron.SkipIfStillRunning(cLogger))
	}

	if opts.withDelay {
		chain = append(chain, cron.DelayIfStillRunning(cLogger))
	}

	if opts.withRecover {
		chain = append(chain, cron.Recover(cLogger))
	}

	if len(chain) > 0 {
		cronOpts = append(cronOpts, cron.WithChain(chain...))
	}

	c := cron.New(cronOpts...)

	return &Cron{
		cron:         c,
		jobs:         make([]Job, 0, 8),
		logger:       opts.logger,
		showFuncName: opts.showFuncName,
		logLevel:     opts.logLevel,
	}, nil
}

func (c *Cron) RegisterJobs(jobs ...Job) {
	c.jobs = append(c.jobs, jobs...)
}

func (c *Cron) Start(ctx context.Context) error {
	total, allowed, onStart := c.summarizeJobs()

	c.logger.Info("cron: starting",
		slog.Group("jobs",
			"total", total,
			"allowed", allowed,
			"onStart", onStart),
	)

	if onStart > 0 {
		c.logger.Info("cron: running on start jobs")
	}

	for _, job := range c.jobs {
		if job.Config.Allowed && job.Config.RunOnStart {
			c.runJob(ctx, job.Fn)
		}
	}

	if onStart > 0 {
		c.logger.Info("cron: on start jobs finished")
	}

	for _, job := range c.jobs {
		if job.Config.Allowed {
			for _, spec := range job.Config.Spec {
				_, err := c.cron.AddFunc(spec, func() {
					c.runJob(ctx, job.Fn)
				})
				if err != nil {
					c.logger.Error("cron: failed to schedule job", "spec", spec, sl.Error(err))

					return fmt.Errorf("cron.AddFunc: %w", err)
				}
			}
		}
	}

	c.logger.Info("cron: started")
	c.cron.Start()

	defer func() {
		c.cron.Stop()
		c.logger.Info("cron: stopped")
	}()

	<-ctx.Done()

	return nil
}

func (c *Cron) runJob(ctx context.Context, fn func(context.Context) error) {
	t0 := time.Now()

	var attrs []slog.Attr
	if c.showFuncName {
		attrs = append(attrs, slog.String("method", funcName(fn)))
	}

	c.logger.LogAttrs(ctx, c.logLevel, "cron job started", attrs...)

	if err := fn(ctx); err != nil && !errors.Is(err, context.Canceled) {
		c.logger.LogAttrs(ctx, slog.LevelError, "cron job failed",
			append(attrs, sl.Since(t0), sl.Error(err))...,
		)

		return
	}

	c.logger.LogAttrs(ctx, c.logLevel, "cron job ended",
		append(attrs, sl.Since(t0))...,
	)
}

func (c *Cron) summarizeJobs() (int, int, int) {
	total := len(c.jobs)

	var (
		allowed, onStart int
	)

	for _, j := range c.jobs {
		if !j.Config.Allowed {
			continue
		}

		allowed++

		if j.Config.RunOnStart {
			onStart++
		}
	}

	return total, allowed, onStart
}

var reGenerics = regexp.MustCompile(`\[[^\]]*\]`)

func funcName(fn any) string {
	if fn == nil {
		return ""
	}

	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	name = strings.TrimSuffix(name, "-fm")

	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}

	name = strings.ReplaceAll(name, "(*", "(")
	name = strings.ReplaceAll(name, ")", "")
	name = strings.ReplaceAll(name, "*", "")

	if strings.Contains(name, "[") {
		name = reGenerics.ReplaceAllString(name, "")
	}

	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", ".")
	}

	if !strings.Contains(name, ".") {
		return name
	}

	name = strings.ReplaceAll(name, "(", "")
	name = strings.ReplaceAll(name, ")", "")

	return name
}
