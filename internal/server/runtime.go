package server

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lynn/claudia-gateway/internal/config"
	"github.com/lynn/claudia-gateway/internal/gatewaymetrics"
	"github.com/lynn/claudia-gateway/internal/providerlimits"
	"github.com/lynn/claudia-gateway/internal/routing"
	"github.com/lynn/claudia-gateway/internal/tokens"
)

// Runtime mirrors src/runtime.ts RuntimeState.
type Runtime struct {
	log              *slog.Logger
	gatewayPath      string
	mu               sync.RWMutex
	gatewayMtime     time.Time
	freeTierMtime    time.Time
	resolved         *config.Resolved
	tokens           *tokens.Store
	routing          *routing.Policy
	metrics          *gatewaymetrics.Store // optional; nil when disabled or init failed
	upstreamOverride string                // non-empty: after each yaml load, patch upstream base + health (supervised BiFrost)

	toolRouterMu      sync.Mutex
	toolRouterModel   string
	toolRouterAt      time.Time
	toolRouterLastErr string
}

func NewRuntime(gatewayPath string, log *slog.Logger) (*Runtime, error) {
	return NewRuntimeWithUpstreamOverride(gatewayPath, log, "")
}

// NewRuntimeWithUpstreamOverride loads gateway.yaml; if upstreamOverride is set (e.g. http://127.0.0.1:8080),
// it replaces upstream.base_url and health probe URL on every reload (supervised BiFrost).
func NewRuntimeWithUpstreamOverride(gatewayPath string, log *slog.Logger, upstreamOverride string) (*Runtime, error) {
	res, err := config.LoadGatewayYAML(gatewayPath, log)
	if err != nil {
		return nil, err
	}
	res, err = config.EnsureGeneratedUpstreamAPIKey(gatewayPath, res, log)
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
	if res.MetricsEnabled {
		if s, err := gatewaymetrics.Open(res.MetricsSQLitePath, res.MetricsMigrationsDir, log); err != nil {
			if log != nil {
				log.Error("gateway metrics init failed; continuing without SQLite metrics", "err", err,
					"sqlite", res.MetricsSQLitePath, "migrations_dir", res.MetricsMigrationsDir)
			}
		} else {
			rt.metrics = s
		}
	}
	if st, err := os.Stat(gatewayPath); err == nil {
		rt.gatewayMtime = st.ModTime()
	}
	if res != nil && res.ProviderFreeTierPath != "" {
		if st, err := os.Stat(res.ProviderFreeTierPath); err == nil {
			rt.freeTierMtime = st.ModTime()
		}
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

	gst, err := os.Stat(rt.gatewayPath)
	if err != nil {
		if rt.log != nil {
			rt.log.Error("gateway config missing", "path", rt.gatewayPath, "err", err)
		}
		return
	}
	ftPath := ""
	var ftTime time.Time
	if rt.resolved != nil {
		ftPath = rt.resolved.ProviderFreeTierPath
	}
	if ftPath != "" {
		if st, err := os.Stat(ftPath); err == nil {
			ftTime = st.ModTime()
		}
	}
	if gst.ModTime().Equal(rt.gatewayMtime) && ftTime.Equal(rt.freeTierMtime) {
		return
	}

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
	rt.gatewayMtime = gst.ModTime()
	if next.ProviderFreeTierPath != "" {
		if st, err := os.Stat(next.ProviderFreeTierPath); err == nil {
			rt.freeTierMtime = st.ModTime()
		}
	} else {
		rt.freeTierMtime = time.Time{}
	}
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

// Metrics returns the SQLite metrics recorder, or nil when metrics are disabled or failed to open.
func (rt *Runtime) Metrics() gatewaymetrics.Recorder {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.metrics
}

// MetricsStore returns the metrics SQLite store for admin read APIs, or nil.
func (rt *Runtime) MetricsStore() *gatewaymetrics.Store {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.metrics
}

// LimitsGuard returns an admission guard combining the parsed limits spec with live metrics.
// Returns nil when no limits spec is configured or the metrics store is unavailable; callers
// treat nil as "no enforcement".
// NoteToolRouterAttempt records the last tool-router upstream call (for admin visibility).
func (rt *Runtime) NoteToolRouterAttempt(model string, err error) {
	rt.toolRouterMu.Lock()
	defer rt.toolRouterMu.Unlock()
	rt.toolRouterModel = strings.TrimSpace(model)
	rt.toolRouterAt = time.Now().UTC()
	rt.toolRouterLastErr = ""
	if err != nil {
		rt.toolRouterLastErr = err.Error()
		if len(rt.toolRouterLastErr) > 220 {
			rt.toolRouterLastErr = rt.toolRouterLastErr[:220]
		}
	}
}

// ToolRouterLast returns the last router attempt metadata (best-effort; in-process only).
func (rt *Runtime) ToolRouterLast() (model string, at time.Time, errMsg string) {
	rt.toolRouterMu.Lock()
	defer rt.toolRouterMu.Unlock()
	return rt.toolRouterModel, rt.toolRouterAt, rt.toolRouterLastErr
}

func (rt *Runtime) LimitsGuard() *providerlimits.Guard {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	if rt.resolved == nil || rt.resolved.ProviderLimitsSpec == nil || rt.metrics == nil {
		return nil
	}
	return &providerlimits.Guard{
		Cfg:   rt.resolved.ProviderLimitsSpec,
		Usage: metricsUsageAdapter{store: rt.metrics},
	}
}

// metricsUsageAdapter wraps *gatewaymetrics.Store so it satisfies providerlimits.UsageSource
// without providerlimits importing the SQLite package.
type metricsUsageAdapter struct{ store *gatewaymetrics.Store }

func (a metricsUsageAdapter) UsageForModelWindow(ctx context.Context, modelID string, start, end time.Time) (int64, int64, error) {
	if a.store == nil {
		return 0, 0, nil
	}
	u, err := a.store.UsageForModelWindow(ctx, modelID, start, end)
	if err != nil {
		return 0, 0, err
	}
	return u.Calls, u.EstTokens, nil
}

// CloseMetrics closes the SQLite metrics store if it was opened (tests and graceful shutdown).
func (rt *Runtime) CloseMetrics() {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.metrics != nil {
		_ = rt.metrics.Close()
		rt.metrics = nil
	}
}

func (rt *Runtime) UpstreamAPIKey() string {
	rt.mu.RLock()
	r := rt.resolved
	rt.mu.RUnlock()
	if r == nil {
		return ""
	}
	if v := strings.TrimSpace(os.Getenv(r.UpstreamAPIKeyEnv)); v != "" {
		return v
	}
	return strings.TrimSpace(r.UpstreamAPIKey)
}
