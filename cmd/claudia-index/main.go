// claudia-index is the v0.2 workspace file indexer for the Claudia Gateway.
//
// It walks configured roots, applies .claudiaignore + .gitignore + binary
// detection, hashes whole files, and POSTs them to /v1/ingest. Watching uses
// fsnotify for incremental updates.
//
// Usage:
//
//	claudia-index --config .claudia/indexer.config.yaml [--root path]... [--gateway-url URL]
//
// Environment:
//
//	CLAUDIA_GATEWAY_URL    base URL of the gateway (e.g. http://127.0.0.1:8080)
//	CLAUDIA_GATEWAY_TOKEN  bearer token (required)
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/lynn/claudia-gateway/internal/indexer"
)

type rootList []string

func (r *rootList) String() string { return strings.Join(*r, ",") }
func (r *rootList) Set(v string) error {
	*r = append(*r, v)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "claudia-index:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		cfgPath     string
		gatewayURL  string
		roots       rootList
		oneShot     bool
		showVersion bool
	)
	flag.StringVar(&cfgPath, "config", "", "path to indexer YAML config")
	flag.StringVar(&gatewayURL, "gateway-url", "", "override gateway URL (env "+indexer.EnvGatewayURL+")")
	flag.Var(&roots, "root", "watch root (repeatable; overrides config 'roots')")
	flag.BoolVar(&oneShot, "one-shot", false, "perform a single scan + ingest pass and exit")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println("claudia-index v0.2.0")
		return nil
	}

	fc, err := indexer.LoadFile(cfgPath)
	if err != nil {
		return err
	}
	cfg, err := indexer.Resolve(fc, os.Getenv, indexer.Overrides{
		GatewayURL: gatewayURL,
		Roots:      roots,
	})
	if err != nil {
		return err
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	client := indexer.NewGatewayClient(cfg.GatewayURL, cfg.Token, cfg.RequestTimeout)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ix := indexer.New(cfg, client, log)
	if _, err := ix.FetchAndLogConfig(ctx); err != nil {
		log.Warn("continuing despite config fetch failure", "err", err)
	}
	if _, err := ix.EnqueueInitialScan(ctx); err != nil {
		return err
	}

	if oneShot {
		// Run workers until the queue drains, then stop.
		drainCtx, drainCancel := context.WithCancel(ctx)
		go func() {
			for {
				if ix.Queue().Len() == 0 {
					drainCancel()
					return
				}
				select {
				case <-ctx.Done():
					drainCancel()
					return
				default:
				}
			}
		}()
		ix.RunWorkers(drainCtx)
		ix.Queue().Close()
		return nil
	}

	doneWorkers := make(chan struct{})
	go func() { defer close(doneWorkers); ix.RunWorkers(ctx) }()
	if err := ix.RunWatchers(ctx); err != nil {
		log.Error("watcher exited", "err", err)
	}
	ix.Queue().Close()
	<-doneWorkers
	return nil
}
