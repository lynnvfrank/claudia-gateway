package server

import (
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/lynn/claudia-gateway/internal/config"
	"github.com/lynn/claudia-gateway/internal/routing"
	"github.com/lynn/claudia-gateway/internal/tokens"
)

// Runtime mirrors src/runtime.ts RuntimeState.
type Runtime struct {
	log              *slog.Logger
	gatewayPath      string
	mu               sync.RWMutex
	gatewayMtime     time.Time
	resolved         *config.Resolved
	tokens           *tokens.Store
	routing          *routing.Policy
	upstreamOverride string // non-empty: after each yaml load, patch litellm base + health (Phase 3 supervise)
}

func NewRuntime(gatewayPath string, log *slog.Logger) (*Runtime, error) {
	return NewRuntimeWithUpstreamOverride(gatewayPath, log, "")
}

// NewRuntimeWithUpstreamOverride loads gateway.yaml; if upstreamOverride is set (e.g. http://127.0.0.1:8080),
// it replaces litellm.base_url and health probe URL on every reload (supervised BiFrost).
func NewRuntimeWithUpstreamOverride(gatewayPath string, log *slog.Logger, upstreamOverride string) (*Runtime, error) {
	res, err := config.LoadGatewayYAML(gatewayPath, log)
	if err != nil {
		return nil, err
	}
	if upstreamOverride != "" {
		res = config.CloneResolved(res)
		config.PatchResolvedUpstream(res, upstreamOverride)
	}
	rt := &Runtime{
		log:              log,
		gatewayPath:      gatewayPath,
		upstreamOverride: upstreamOverride,
		resolved:         res,
		tokens:           tokens.NewStore(res.TokensPath, log),
		routing:          routing.NewPolicy(res.RoutingPolicyPath, log),
	}
	if st, err := os.Stat(gatewayPath); err == nil {
		rt.gatewayMtime = st.ModTime()
	}
	return rt, nil
}

func (rt *Runtime) applyUpstreamOverride(res *config.Resolved) *config.Resolved {
	if rt.upstreamOverride == "" {
		return res
	}
	cp := config.CloneResolved(res)
	config.PatchResolvedUpstream(cp, rt.upstreamOverride)
	return cp
}

func (rt *Runtime) Sync() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	st, err := os.Stat(rt.gatewayPath)
	if err != nil {
		if rt.log != nil {
			rt.log.Error("gateway config missing", "path", rt.gatewayPath, "err", err)
		}
		return
	}
	if st.ModTime().Equal(rt.gatewayMtime) {
		return
	}
	rt.gatewayMtime = st.ModTime()

	next, err := config.LoadGatewayYAML(rt.gatewayPath, rt.log)
	if err != nil {
		if rt.log != nil {
			rt.log.Error("failed to reload gateway.yaml", "path", rt.gatewayPath, "err", err)
		}
		return
	}
	pathsChanged := next.TokensPath != rt.resolved.TokensPath ||
		next.RoutingPolicyPath != rt.resolved.RoutingPolicyPath
	rt.resolved = rt.applyUpstreamOverride(next)
	if pathsChanged {
		rt.tokens = tokens.NewStore(next.TokensPath, rt.log)
		rt.routing = routing.NewPolicy(next.RoutingPolicyPath, rt.log)
	}
	if rt.log != nil {
		rt.log.Info("reloaded gateway.yaml", "path", rt.gatewayPath)
	}
}

func (rt *Runtime) Snapshot() (*config.Resolved, *tokens.Store, *routing.Policy) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.resolved, rt.tokens, rt.routing
}

func (rt *Runtime) UpstreamAPIKey() string {
	rt.mu.RLock()
	r := rt.resolved
	rt.mu.RUnlock()
	if r == nil {
		return ""
	}
	return os.Getenv(r.LitellmAPIKeyEnv)
}
