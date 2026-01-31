package app

import (
	"context"
	"time"

	"github.com/five82/flyer/internal/spindle"
	"github.com/five82/flyer/internal/state"
)

const (
	defaultPollInterval = 2 * time.Second
	maxBackoff          = 30 * time.Second
)

// StartPoller launches a background goroutine that refreshes the store at a
// fixed cadence with exponential backoff on failures. It returns immediately.
func StartPoller(ctx context.Context, store *state.Store, client *spindle.Client, interval time.Duration) {
	if interval <= 0 {
		interval = defaultPollInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var consecutiveFailures int
		var lastPollTime time.Time

		for {
			// Calculate backoff based on consecutive failures
			backoff := calculateBackoff(consecutiveFailures, interval)

			// Skip poll if within backoff period
			if consecutiveFailures > 0 && time.Since(lastPollTime) < backoff {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					continue
				}
			}

			lastPollTime = time.Now()
			err := refresh(ctx, store, client)
			if err != nil {
				consecutiveFailures++
			} else {
				consecutiveFailures = 0
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// calculateBackoff returns the backoff duration for the given failure count.
// Uses exponential backoff: interval * 2^failures, capped at maxBackoff.
func calculateBackoff(failures int, baseInterval time.Duration) time.Duration {
	if failures <= 0 {
		return baseInterval
	}
	// Exponential backoff: baseInterval * 2^failures
	backoff := baseInterval * time.Duration(1<<failures)
	if backoff > maxBackoff {
		return maxBackoff
	}
	return backoff
}

func refresh(ctx context.Context, store *state.Store, client *spindle.Client) error {
	status, err := client.FetchStatus(ctx)
	if err != nil {
		store.Update(nil, nil, err)
		return err
	}
	queue, err := client.FetchQueue(ctx)
	if err != nil {
		store.Update(nil, nil, err)
		return err
	}
	store.Update(status, queue, nil)
	return nil
}
