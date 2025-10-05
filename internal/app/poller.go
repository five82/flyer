package app

import (
	"context"
	"log"
	"time"

	"github.com/five82/flyer/internal/spindle"
	"github.com/five82/flyer/internal/state"
)

const defaultPollInterval = 2 * time.Second

// StartPoller launches a background goroutine that refreshes the store at a
// fixed cadence. It returns immediately.
func StartPoller(ctx context.Context, store *state.Store, client *spindle.Client, interval time.Duration) {
	if interval <= 0 {
		interval = defaultPollInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			refresh(ctx, store, client)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func refresh(ctx context.Context, store *state.Store, client *spindle.Client) {
	status, err := client.FetchStatus(ctx)
	if err != nil {
		store.Update(nil, nil, err)
		log.Printf("status poll failed: %v", err)
		return
	}
	queue, err := client.FetchQueue(ctx)
	if err != nil {
		store.Update(nil, nil, err)
		log.Printf("queue poll failed: %v", err)
		return
	}
	store.Update(status, queue, nil)
}
