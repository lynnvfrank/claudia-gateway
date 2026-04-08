package main

import (
	"fmt"
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
	if len(args) > 0 && (args[0] == "-version" || args[0] == "--version") {
		fmt.Printf("claudia %s\ncommit %s\nbuild date %s\n", version, commit, date)
		return
	}
	if len(args) == 0 || (args[0] != "serve" && args[0] != "supervise" && args[0] != "desktop") {
		for _, a := range args {
			if a == "-h" || a == "--help" {
				printHelp()
				return
			}
		}
	}
	if len(args) > 0 {
		switch args[0] {
		case "serve", "supervise":
			runServe(args[1:], false)
			return
		case "desktop":
			runServe(args[1:], true)
			return
		case "help", "-h", "--help":
			printHelp()
			return
		}
	}
	runGateway(args)
}

func printHelp() {
	fmt.Print(`Claudia gateway (Go)

Usage:
  claudia [flags]              Run HTTP gateway only (uses gateway.yaml upstream URL).
  claudia serve [flags]        Optional Qdrant + BiFrost subprocesses, then gateway (overrides upstream to local BiFrost).
  claudia desktop [flags]      Same as serve plus native webview to /ui/panel (requires: go build -tags desktop, CGO).
  claudia help
  claudia -version            Print build version (release builds embed tag/commit via GoReleaser)

Obtain the BiFrost binary from upstream releases or build from source (see docs/supervisor.md).
Default gateway config: $CLAUDIA_GATEWAY_CONFIG or ./config/gateway.yaml

`)
}

func buildLogger(gatewayPath string) *slog.Logger {
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
	return slog.New(slog.NewTextHandler(os.Stdout, hopts))
}
