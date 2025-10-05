package app

import (
	"context"
	"fmt"
	"time"

	"github.com/five82/flyer/internal/config"
	"github.com/five82/flyer/internal/spindle"
	"github.com/five82/flyer/internal/state"
	"github.com/five82/flyer/internal/ui"
)

// Options configure the Flyer application.
type Options struct {
	ConfigPath string
	PollEvery  int // seconds; zero uses default
}

// Run boots the Flyer TUI until the context is cancelled.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("load spindle config: %w", err)
	}

	client, err := spindle.NewClient(cfg.APIBind)
	if err != nil {
		return fmt.Errorf("init spindle client: %w", err)
	}

	store := &state.Store{}

	interval := defaultPollInterval
	if opts.PollEvery > 0 {
		interval = time.Duration(opts.PollEvery) * time.Second
	}

	StartPoller(ctx, store, client, interval)

	uiOpts := ui.Options{
		Store:        store,
		LogPath:      cfg.DaemonLogPath(),
		Config:       cfg,
		RefreshEvery: interval,
	}

	return ui.Run(ctx, uiOpts)
}
