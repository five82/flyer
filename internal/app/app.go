package app

import (
	"context"
	"fmt"
	"time"

	"github.com/five82/flyer/internal/config"
	"github.com/five82/flyer/internal/prefs"
	"github.com/five82/flyer/internal/spindle"
	"github.com/five82/flyer/internal/state"
	"github.com/five82/flyer/internal/ui/tea"
)

// Options configure the Flyer application.
type Options struct {
	ConfigPath string
	PrefsPath  string // empty uses default ~/.config/flyer/prefs.toml
	PollEvery  int    // seconds; zero uses default
}

// Run boots the Flyer TUI until the context is cancelled.
func Run(ctx context.Context, opts Options) error {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("load spindle config: %w", err)
	}

	userPrefs := prefs.Load(opts.PrefsPath)

	client, err := spindle.NewClient(cfg.APIBind)
	if err != nil {
		return fmt.Errorf("init spindle client: %w", err)
	}

	store := &state.Store{}

	interval := defaultPollInterval
	if opts.PollEvery > 0 {
		interval = time.Duration(opts.PollEvery) * time.Second
	}

	if err := ensureSpindleAvailable(ctx, client); err != nil {
		return err
	}

	// Start background poller
	StartPoller(ctx, store, client, interval)

	// Do initial refresh to populate store before UI starts
	refresh(ctx, store, client)

	teaOpts := tea.Options{
		Context:   ctx,
		Client:    client,
		Store:     store,
		Config:    &cfg,
		PollTick:  interval,
		ThemeName: userPrefs.Theme,
		PrefsPath: opts.PrefsPath,
	}
	return tea.Run(teaOpts)
}

const initialConnectTimeout = 3 * time.Second

func ensureSpindleAvailable(ctx context.Context, client *spindle.Client) error {
	if client == nil {
		return fmt.Errorf("spindle daemon unavailable: no client")
	}
	checkCtx := ctx
	var cancel context.CancelFunc
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > initialConnectTimeout {
		checkCtx, cancel = context.WithTimeout(ctx, initialConnectTimeout)
		defer cancel()
	}
	if _, err := client.FetchStatus(checkCtx); err != nil {
		return fmt.Errorf("spindle daemon unavailable: %w", err)
	}
	return nil
}
