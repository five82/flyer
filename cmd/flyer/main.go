package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/five82/flyer/internal/app"
)

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String("config", "", "override spindle config path (optional)")
	pollSeconds := flag.Int("poll", 0, "refresh interval in seconds (optional, defaults to 2s)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	opts := app.Options{ConfigPath: *configPath}
	if poll := *pollSeconds; poll > 0 {
		opts.PollEvery = poll
	}

	if err := app.Run(ctx, opts); err != nil {
		fmt.Fprintf(os.Stderr, "flyer: %v\n", err)
		return 1
	}
	return 0
}
