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
	// Optional .env in the process working directory (e.g. repo root); production
	// typically injects env without a file. Missing .env is not an error.
	_ = godotenv.Load()

	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "-version" || args[0] == "--version") {
		fmt.Printf("claudia %s\ncommit %s\nbuild date %s\n", version, commit, date)
		return
	}
	if len(args) == 0 || (args[0] != "serve" && args[0] != "supervise") {
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
			runServe(args[1:])
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
