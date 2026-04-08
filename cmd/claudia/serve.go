package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lynn/claudia-gateway/internal/config"
	"github.com/lynn/claudia-gateway/internal/server"
	"github.com/lynn/claudia-gateway/internal/supervisor"
)

func panelURLFromListenAddr(ln net.Addr) string {
	addr := ln.String()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://127.0.0.1:3000/ui/panel"
	}
	if host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return fmt.Sprintf("http://[%s]:%s/ui/panel", host, port)
	}
	return fmt.Sprintf("http://%s:%s/ui/panel", host, port)
}

func runServe(args []string, openWebview bool) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to gateway.yaml (default: $CLAUDIA_GATEWAY_CONFIG or ./config/gateway.yaml)")
	listen := fs.String("listen", "", "Override Claudia listen address (host:port or :port)")

	bifrostBin := fs.String("bifrost-bin", defaultSupervisorBifrostBin(), "BiFrost HTTP binary (PATH or path; defaults to bifrost-http next to this executable if present, else bifrost on PATH)")
	bifrostConfig := fs.String("bifrost-config", "config/bifrost.config.json", "Source bifrost.config.json (copied to data dir as config.json)")
	bifrostDataDir := fs.String("bifrost-data-dir", "data/bifrost", "BiFrost working directory (created; SQLite and config live here)")
	bifrostBind := fs.String("bifrost-bind", "127.0.0.1", "BiFrost bind address (-host)")
	bifrostPort := fs.Int("bifrost-port", 8080, "BiFrost listen port (-port)")
	bifrostLogLevel := fs.String("bifrost-log-level", "info", "BiFrost -log-level (debug, info, warn, error)")
	bifrostLogStyle := fs.String("bifrost-log-style", "json", "BiFrost -log-style (json or pretty)")
	upstreamHost := fs.String("upstream-host", "127.0.0.1", "Host for gateway upstream.base_url (Claudia → BiFrost); use 127.0.0.1 when bifrost-bind is 0.0.0.0")
	waitTimeout := fs.Duration("wait-bifrost", 60*time.Second, "Max time to poll BiFrost /health before exit")
	noWait := fs.Bool("no-wait-bifrost", false, "Skip readiness poll (not recommended)")

	qdrantBin := fs.String("qdrant-bin", defaultSupervisorQdrantBin(), "Qdrant binary (PATH or path); empty skips Qdrant (defaults to qdrant next to this executable if present)")
	qdrantStorage := fs.String("qdrant-storage", "data/qdrant", "Qdrant storage directory (created)")
	qdrantBind := fs.String("qdrant-bind", "127.0.0.1", "Qdrant QDRANT__SERVICE__HOST")
	qdrantHTTPPort := fs.Int("qdrant-http-port", 6333, "Qdrant HTTP port")
	qdrantGRPCPort := fs.Int("qdrant-grpc-port", 6334, "Qdrant gRPC port")
	qdrantHealthHost := fs.String("qdrant-health-host", "127.0.0.1", "Host for GET /readyz probe (use 127.0.0.1 when qdrant-bind is 0.0.0.0)")
	waitQdrant := fs.Duration("wait-qdrant", 60*time.Second, "Max time to poll Qdrant /readyz before exit")
	noWaitQdrant := fs.Bool("no-wait-qdrant", false, "Skip Qdrant readiness poll")

	_ = fs.Parse(args)

	path := strings.TrimSpace(*configPath)
	if path == "" {
		var err error
		path, err = config.ResolveGatewayConfigPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "claudia serve:", err)
			os.Exit(2)
		}
	}

	log := buildLogger(path)
	upstreamURL := fmt.Sprintf("http://%s:%d", strings.TrimSpace(*upstreamHost), *bifrostPort)
	healthURL := fmt.Sprintf("http://%s:%d/health", strings.TrimSpace(*upstreamHost), *bifrostPort)

	childCtx, stopChildren := context.WithCancel(context.Background())

	var qdrantProc *exec.Cmd
	var qdrantWait chan error
	qBin := strings.TrimSpace(*qdrantBin)
	if qBin != "" {
		qcfg := supervisor.QdrantConfig{
			Bin:        qBin,
			StorageDir: *qdrantStorage,
			BindHost:   strings.TrimSpace(*qdrantBind),
			HTTPPort:   *qdrantHTTPPort,
			GRPCPort:   *qdrantGRPCPort,
		}
		var err error
		qdrantProc, err = supervisor.StartQdrant(childCtx, qcfg, log)
		if err != nil {
			stopChildren()
			fmt.Fprintf(os.Stderr, "claudia serve: %v\n", err)
			os.Exit(1)
		}
		qdrantWait = make(chan error, 1)
		go func() {
			qdrantWait <- qdrantProc.Wait()
		}()
		if !*noWaitQdrant {
			qHealth := fmt.Sprintf("http://%s:%d/readyz", strings.TrimSpace(*qdrantHealthHost), *qdrantHTTPPort)
			wCtx, wCancel := context.WithTimeout(context.Background(), *waitQdrant)
			err := supervisor.WaitHealthy(wCtx, qHealth, *waitQdrant, log)
			wCancel()
			if err != nil {
				stopChildren()
				<-qdrantWait
				fmt.Fprintf(os.Stderr, "claudia serve: qdrant not healthy: %v\n", err)
				os.Exit(1)
			}
		}
	}

	bcfg := supervisor.BifrostConfig{
		Bin:        *bifrostBin,
		ConfigJSON: *bifrostConfig,
		DataDir:    *bifrostDataDir,
		BindHost:   strings.TrimSpace(*bifrostBind),
		Port:       *bifrostPort,
		LogLevel:   strings.TrimSpace(*bifrostLogLevel),
		LogStyle:   strings.TrimSpace(*bifrostLogStyle),
	}
	proc, err := supervisor.StartBifrost(childCtx, bcfg, log)
	if err != nil {
		stopChildren()
		if qdrantWait != nil {
			<-qdrantWait
		}
		fmt.Fprintf(os.Stderr, "claudia serve: %v\n", err)
		if errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "executable file not found") {
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "No BiFrost HTTP binary found (place bifrost-http next to claudia, PATH, or pass -bifrost-bin). From repo root:")
			fmt.Fprintln(os.Stderr, "  make install")
			fmt.Fprintln(os.Stderr, "  ./claudia serve -bifrost-bin ./bin/bifrost-http")
			fmt.Fprintln(os.Stderr, "Or: make package-personal  (full folder with bifrost-http + qdrant + config)")
			fmt.Fprintln(os.Stderr, "See docs/supervisor.md — Obtaining the BiFrost binary.")
		}
		os.Exit(1)
	}
	bifrostWaitErr := make(chan error, 1)
	go func() {
		bifrostWaitErr <- proc.Wait()
	}()

	if !*noWait {
		wCtx, wCancel := context.WithTimeout(context.Background(), *waitTimeout)
		err := supervisor.WaitHealthy(wCtx, healthURL, *waitTimeout, log)
		wCancel()
		if err != nil {
			stopChildren()
			if qdrantWait != nil {
				<-qdrantWait
			}
			<-bifrostWaitErr
			fmt.Fprintf(os.Stderr, "claudia serve: bifrost not healthy: %v\n", err)
			os.Exit(1)
		}
	}

	rt, err := server.NewRuntimeWithUpstreamOverride(path, log, upstreamURL)
	if err != nil {
		stopChildren()
		if qdrantWait != nil {
			<-qdrantWait
		}
		<-bifrostWaitErr
		fmt.Fprintf(os.Stderr, "claudia serve: load gateway config: %v\n", err)
		os.Exit(1)
	}

	res, _, _ := rt.Snapshot()
	addr := server.ListenAddrOverride(res, *listen)

	qdrantHTTP := ""
	if qBin != "" {
		qdrantHTTP = fmt.Sprintf("%s:%d", strings.TrimSpace(*qdrantHealthHost), *qdrantHTTPPort)
	}
	overlay := &server.StatusOverlay{
		EffectiveListen: addr,
		Supervisor: &server.SupervisorInfo{
			BifrostListen:    fmt.Sprintf("%s:%d", strings.TrimSpace(*bifrostBind), *bifrostPort),
			QdrantSupervised: qBin != "",
			QdrantHTTP:       qdrantHTTP,
		},
	}
	h := server.NewMux(rt, log, overlay, server.NewUIOptions())

	rootCtx, stopRoot := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopRoot()

	srv := &http.Server{Handler: h}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		stopChildren()
		if qdrantWait != nil {
			<-qdrantWait
		}
		<-bifrostWaitErr
		if log != nil {
			log.Error("listen", "addr", addr, "err", err)
		}
		fmt.Fprintf(os.Stderr, "claudia serve: listen %s: %v\n", addr, err)
		os.Exit(1)
	}
	panelURL := panelURLFromListenAddr(ln.Addr())

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- srv.Serve(ln)
	}()

	go func() {
		<-rootCtx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shCtx); err != nil && log != nil {
			log.Warn("http shutdown", "err", err)
		}
	}()

	log.Info("claudia serve: gateway listening", "addr", ln.Addr().String(), "ui", panelURL, "upstream", upstreamURL, "bifrost_data", *bifrostDataDir, "qdrant_supervised", qBin != "", "config", path)

	runDesktopWebview(openWebview, panelURL, stopRoot, rootCtx)

	serveErr := <-serveErrCh

	stopChildren()
	if qdrantWait != nil {
		if werr := <-qdrantWait; werr != nil && log != nil {
			log.Debug("qdrant process finished", "err", werr)
		}
	}
	if werr := <-bifrostWaitErr; werr != nil && log != nil {
		log.Debug("bifrost process finished", "err", werr)
	}

	if serveErr != nil && serveErr != http.ErrServerClosed {
		log.Error("http server", "err", serveErr)
		stopRoot()
		os.Exit(1)
	}
}
