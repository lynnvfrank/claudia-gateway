package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/lynn/claudia-gateway/internal/config"
	"github.com/lynn/claudia-gateway/internal/server"
)

func runGateway(args []string) {
	fs := flag.NewFlagSet("claudia", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to gateway.yaml (default: $CLAUDIA_GATEWAY_CONFIG or ./config/gateway.yaml)")
	listen := fs.String("listen", "", "Override listen address (default: gateway.listen_host:listen_port from yaml)")
	_ = fs.Parse(args)

	path := strings.TrimSpace(*configPath)
	if path == "" {
		var err error
		path, err = config.ResolveGatewayConfigPath()
		if err != nil {
			fmt.Fprintln(os.Stderr, "claudia:", err)
			os.Exit(2)
		}
	}

	log := buildLogger(path)
	rt, err := server.NewRuntime(path, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "claudia: load gateway config: %v\n", err)
		os.Exit(1)
	}

	res, _, _ := rt.Snapshot()
	addr := server.ListenAddrOverride(res, *listen)
	h := server.NewMux(rt, log, &server.StatusOverlay{EffectiveListen: addr}, server.NewUIOptions())
	log.Info("claudia (go) listening", "addr", addr, "upstream", res.UpstreamBaseURL, "config", path)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Error("server exit", "err", err)
		os.Exit(1)
	}
}
