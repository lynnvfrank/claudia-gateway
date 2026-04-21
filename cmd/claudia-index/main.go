// claudia-index is the v0.4 workspace file indexer for the Claudia Gateway.
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
//	CLAUDIA_GATEWAY_URL    base URL of the gateway (default port 3000)
//	CLAUDIA_GATEWAY_TOKEN  bearer token; must equal a token: entry in
//	                       config/tokens.yaml on the gateway side
//
// On startup the binary loads `env` and then `.env` (later wins) from the
// current working directory, mirroring the main `claudia` binary so operators
// can keep one secrets file for both.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
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
	// Load env files from cwd (missing files ignored) before reading flags so
	// operators can stash CLAUDIA_GATEWAY_URL/_TOKEN in `.env` next to the
	// gateway's own .env. Matches cmd/claudia behavior.
	_ = godotenv.Load("env")
	_ = godotenv.Load(".env")

	var (
		cfgPath     string
		gatewayURL  string
		roots       rootList
		oneShot     bool
		showVersion bool
	)
	flag.StringVar(&cfgPath, "config", "", "optional indexer YAML merged after ~/.claudia/indexer.config.yaml and ./.claudia/indexer.config.yaml")
	flag.StringVar(&gatewayURL, "gateway-url", "", "override gateway URL (env "+indexer.EnvGatewayURL+")")
	flag.Var(&roots, "root", "watch root (repeatable; overrides config 'roots')")
	flag.BoolVar(&oneShot, "one-shot", false, "perform a single scan + ingest pass and exit")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println("claudia-index v0.4.1")
		return nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	fc, err := indexer.LoadLayeredConfig(wd, cfgPath)
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

	runID := uuid.NewString()
	baseLog := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	log := baseLog.With("index_run_id", runID, "service", "indexer")
	client := indexer.NewGatewayClient(cfg.GatewayURL, cfg.Token, cfg.RequestTimeout)
	client.IndexRunID = runID

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	ix := indexer.New(cfg, client, log)
	log.Info("indexer run start", "msg", "indexer.run.start", "roots", len(cfg.Roots))
	if _, err := ix.FetchAndLogConfig(ctx); err != nil {
		var he *indexer.HTTPError
		if errors.As(err, &he) && he.Status == 503 && strings.Contains(strings.ToLower(he.Body), "rag is not enabled") {
			return fmt.Errorf("gateway at %s has RAG disabled — set rag.enabled=true in config/gateway.yaml and restart the gateway", cfg.GatewayURL)
		}
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
		log.Info("indexer run done", "msg", "indexer.run.done", "mode", "one-shot")
		return nil
	}

	doneWorkers := make(chan struct{})
	go func() { defer close(doneWorkers); ix.RunWorkers(ctx) }()
	if err := ix.RunWatchers(ctx); err != nil {
		log.Error("watcher exited", "err", err)
	}
	ix.Queue().Close()
	<-doneWorkers
	log.Info("indexer run stopped", "msg", "indexer.run.done", "mode", "watch")
	return nil
}
