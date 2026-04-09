package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/lynn/claudia-gateway/internal/config"
	"github.com/lynn/claudia-gateway/internal/server"
)

func main() {
	// Optional env files in cwd: `env` (repo convention) then `.env` (later wins).
	// Missing files are ignored. Production typically injects env without a file.
	_ = godotenv.Load("env")
	_ = godotenv.Load(".env")

	args := os.Args[1:]
	headless := false
	for len(args) > 0 && (args[0] == "--headless" || args[0] == "-headless") {
		headless = true
		args = args[1:]
	}
	if len(args) > 0 && (args[0] == "-version" || args[0] == "--version") {
		fmt.Printf("claudia %s\ncommit %s\nbuild date %s\n", version, commit, date)
		return
	}

	if len(args) == 0 || (args[0] != "serve" && args[0] != "supervise" && args[0] != "desktop" && args[0] != "gateway") {
		for _, a := range args {
			if a == "-h" || a == "--help" {
				printHelp()
				return
			}
		}
	}

	if len(args) == 0 {
		routeNoSubcommand(headless)
		return
	}

	switch args[0] {
	case "serve", "supervise":
		runServe(args[1:], false)
	case "desktop":
		openUI := true
		if headless {
			openUI = false
		}
		runServe(args[1:], openUI)
	case "gateway":
		runGateway(args[1:])
	case "help", "-h", "--help":
		printHelp()
	default:
		runGateway(args)
	}
}

func routeNoSubcommand(headless bool) {
	if defaultNoSubcommandUsesDesktopUI() {
		runServe(nil, !headless)
		return
	}
	if headless {
		runServe(nil, false)
		return
	}
	runGateway(nil)
}

func printHelp() {
	fmt.Print(`Claudia gateway (Go)

Usage:
  claudia [flags]              With -tags desktop: supervisor (BiFrost + Qdrant + gateway) + web UI. Without: HTTP gateway only (gateway.yaml upstream).
  claudia --headless          Supervisor only, no webview (desktop builds). Non-desktop: same as "claudia serve" with no extra args.
  claudia gateway [flags]     HTTP gateway only; same as the non-desktop default above.
  claudia serve [flags]       Supervisor without webview (explicit).
  claudia desktop [flags]     Supervisor; webview unless combined with leading --headless.
  claudia help
  claudia -version            Print build version (release builds embed tag/commit via GoReleaser)

Obtain the BiFrost binary from upstream releases or build from source (see docs/supervisor.md).
Default gateway config: $CLAUDIA_GATEWAY_CONFIG or ./config/gateway.yaml

`)
}

func buildLogger(gatewayPath string) *slog.Logger {
	return buildLoggerTo(os.Stdout, gatewayPath)
}

func buildLoggerTo(w io.Writer, gatewayPath string) *slog.Logger {
	hopts := &slog.HandlerOptions{}
	if e := os.Getenv("LOG_LEVEL"); e != "" {
		hopts.Level = server.ParseLogLevel(e)
	} else {
		res, err := config.LoadGatewayYAML(gatewayPath, nil)
		lvl := slog.LevelInfo
		if err == nil {
			lvl = server.ParseLogLevel(res.LogLevel)
		}
		hopts.Level = lvl
	}
	return slog.New(slog.NewTextHandler(w, hopts))
}
