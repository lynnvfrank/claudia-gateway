package server

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lynn/claudia-gateway/internal/chat"
	"github.com/lynn/claudia-gateway/internal/config"
	"github.com/lynn/claudia-gateway/internal/upstream"
)

const maxBodyBytes = 25 * 1024 * 1024

// NewMux builds the v0.1 HTTP surface (src/server.ts parity). overlay configures GET /status;
// pass nil in tests; production passes listen address and optional supervisor info.
func NewMux(rt *Runtime, log *slog.Logger, overlay *StatusOverlay) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rt.Sync()
		res, _, _ := rt.Snapshot()
		semver := html.EscapeString(res.Semver)
		vm := html.EscapeString(res.VirtualModelID)
		const page = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>Claudia Gateway</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 48rem; margin: 3rem auto; padding: 0 1rem; line-height: 1.5; color: #1a1a1a; }
    h1 { font-size: 1.5rem; }
    h2 { font-size: 1.1rem; margin-top: 2rem; margin-bottom: 0.5rem; }
    .ok { color: #0d6832; font-weight: 600; }
    .err { color: #a40000; font-weight: 600; }
    .muted { color: #555; }
    code { background: #f4f4f4; padding: 0.15em 0.4em; border-radius: 4px; font-size: 0.9em; }
    ul { padding-left: 1.2rem; }
    a { color: #0b57d0; }
    .prov { margin-top: 1rem; }
    .prov h3 { font-size: 0.95rem; margin: 0 0 0.35rem 0; color: #333; text-transform: none; font-weight: 600; }
    .prov ul { margin: 0; list-style: disc; }
    .prov li { font-family: ui-monospace, monospace; font-size: 0.82rem; }
  </style>
</head>
<body>
  <h1>Claudia Gateway</h1>
  <p class="ok">Up and operational.</p>
  <p>Version <code>%s</code> · Virtual model <code>%s</code></p>
  <h2>OpenAI-compatible API</h2>
  <p class="muted">Send <code>Authorization: Bearer &lt;gateway token&gt;</code> on these routes.</p>
  <ul>
    <li><code>GET /v1/models</code> — list models (virtual Claudia model plus upstream)</li>
    <li><code>POST /v1/chat/completions</code> — chat completions (JSON body; <code>stream: true</code> supported)</li>
  </ul>
  <h2>Other routes</h2>
  <p class="muted">No gateway token required.</p>
  <ul>
    <li><a href="/"><code>GET /</code></a> — gateway index (this page)</li>
    <li><a href="/health"><code>GET /health</code></a> — JSON readiness (upstream proxy probe)</li>
    <li><a href="/status"><code>GET /status</code></a> — gateway and optional supervisor JSON (GUI / ops)</li>
    <li><a href="/ui/models"><code>GET /ui/models</code></a> — same merged model list as <code>/v1/models</code> (for this page and tools)</li>
  </ul>
  <h2>Claudia's Models</h2>
  <p id="models-status" class="muted">Loading models…</p>
  <div id="models-root" hidden></div>
  <script>
(function () {
  var statusEl = document.getElementById("models-status");
  var rootEl = document.getElementById("models-root");
  function showError(msg) {
    statusEl.textContent = "Error: " + msg;
    statusEl.className = "err";
    rootEl.hidden = true;
    rootEl.textContent = "";
  }
  fetch("/ui/models")
    .then(function (res) {
      return res.text().then(function (text) {
        var data = null;
        try {
          data = text ? JSON.parse(text) : null;
        } catch (e) {
          throw new Error("Invalid JSON from server (" + res.status + ")");
        }
        return { res: res, data: data };
      });
    })
    .then(function (x) {
      if (!x.res.ok) {
        var msg = (x.data && x.data.error && x.data.error.message) || ("HTTP " + x.res.status);
        showError(msg);
        return;
      }
      var items = (x.data && x.data.data) || [];
      if (!Array.isArray(items)) {
        showError("Unexpected response shape");
        return;
      }
      statusEl.textContent = items.length + " model(s) from upstream (virtual model included).";
      statusEl.className = "muted";
      var byProv = {};
      for (var i = 0; i < items.length; i++) {
        var m = items[i] || {};
        var id = m.id || "";
        var prov = "other";
        var slash = id.indexOf("/");
        if (slash > 0) prov = id.slice(0, slash);
        else if (m.owned_by) prov = String(m.owned_by);
        if (!byProv[prov]) byProv[prov] = [];
        byProv[prov].push(id);
      }
      var names = Object.keys(byProv).sort();
      rootEl.textContent = "";
      for (var j = 0; j < names.length; j++) {
        var p = names[j];
        var ids = byProv[p].slice().sort();
        var section = document.createElement("section");
        section.className = "prov";
        var h = document.createElement("h3");
        h.textContent = p;
        section.appendChild(h);
        var ul = document.createElement("ul");
        for (var k = 0; k < ids.length; k++) {
          var li = document.createElement("li");
          li.textContent = ids[k];
          ul.appendChild(li);
        }
        section.appendChild(ul);
        rootEl.appendChild(section);
      }
      rootEl.hidden = false;
    })
    .catch(function (e) {
      showError(e.message || String(e));
    });
})();
  </script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(fmt.Sprintf(page, semver, vm)))
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rt.Sync()
		res, _, _ := rt.Snapshot()
		apiKey := rt.UpstreamAPIKey()
		ctx := r.Context()
		ok, st, detail := upstream.ProbeHealth(ctx, res.HealthUpstreamURL, apiKey, healthTimeout(res), log)
		upstreamCheck := map[string]any{
			"ok":     ok,
			"status": st,
		}
		if detail != "" {
			upstreamCheck["detail"] = detail
		}
		checks := map[string]any{
			"upstream": upstreamCheck,
		}
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"degraded": true,
				"status":   "degraded",
				"checks":   checks,
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"checks": checks,
		})
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		handleStatus(w, r, rt, log, overlay)
	})

	mux.HandleFunc("/ui/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rt.Sync()
		res, _, _ := rt.Snapshot()
		writeMergedModelsResponse(w, r.Context(), res, rt.UpstreamAPIKey(), healthTimeout(res), log)
	})

	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleV1Models(w, r, rt, log)
	})

	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleV1Chat(w, r, rt, log)
	})

	return loggingMiddleware(log, mux)
}

func handleV1Models(w http.ResponseWriter, r *http.Request, rt *Runtime, log *slog.Logger) {
	rt.Sync()
	res, tokStore, _ := rt.Snapshot()
	token := bearerToken(r.Header.Get("Authorization"))
	sess := tokStore.Validate(token)
	if token == "" || sess == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "Unauthorized", "type": "invalid_api_key"},
		})
		return
	}
	writeMergedModelsResponse(w, r.Context(), res, rt.UpstreamAPIKey(), healthTimeout(res), log)
}

// writeMergedModelsResponse lists upstream GET /v1/models, prepends the virtual Claudia model, and writes OpenAI-style JSON.
func writeMergedModelsResponse(w http.ResponseWriter, ctx context.Context, res *config.Resolved, apiKey string, timeout time.Duration, log *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	if apiKey == "" {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Missing " + res.UpstreamAPIKeyEnv + " for upstream proxy",
				"type":    "gateway_config",
			},
		})
		return
	}
	st, body, ok := upstream.FetchOpenAIModels(ctx, res.UpstreamBaseURL, apiKey, timeout, log)
	if !ok {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Failed to list models from upstream",
				"type":    "gateway_upstream",
				"status":  st,
			},
		})
		return
	}
	var list map[string]any
	if err := json.Unmarshal(body, &list); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid models response from upstream",
				"type":    "gateway_upstream",
			},
		})
		return
	}
	data, _ := list["data"].([]any)
	if data == nil {
		data = []any{}
	}
	virtual := map[string]any{
		"id":       res.VirtualModelID,
		"object":   "model",
		"created":  time.Now().Unix(),
		"owned_by": "claudia",
	}
	out := append([]any{virtual}, data...)
	_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "data": out})
}

func handleV1Chat(w http.ResponseWriter, r *http.Request, rt *Runtime, log *slog.Logger) {
	rt.Sync()
	res, tokStore, pol := rt.Snapshot()
	token := bearerToken(r.Header.Get("Authorization"))
	sess := tokStore.Validate(token)
	if token == "" || sess == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "Unauthorized", "type": "invalid_api_key"},
		})
		return
	}
	apiKey := rt.UpstreamAPIKey()
	if apiKey == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Missing " + res.UpstreamAPIKeyEnv + " for upstream proxy",
				"type":    "gateway_config",
			},
		})
		return
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil || raw == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "Expected JSON body", "type": "invalid_request"},
		})
		return
	}

	var stream bool
	if s, ok := raw["stream"]; ok {
		_ = json.Unmarshal(s, &stream)
	}

	var clientModel string
	if m, ok := raw["model"]; ok {
		_ = json.Unmarshal(m, &clientModel)
	}

	if log != nil {
		log.Info("chat completion request", "clientModel", clientModel, "stream", stream, "tenant", sess.TenantID)
	}

	ctx := r.Context()
	if clientModel == res.VirtualModelID {
		initial, _ := pol.PickInitialModel(raw, res.FallbackChain, res.VirtualModelID)
		if initial == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"message": "Could not resolve an initial upstream model for the virtual Claudia model (check routing policy and fallback chain).",
					"type":    "gateway_config",
				},
			})
			return
		}
		chat.WithVirtualModelFallback(ctx, w, initial, res.FallbackChain, res.UpstreamBaseURL, apiKey, stream, raw, chatTimeout(res), log)
		return
	}

	if clientModel == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "Missing model", "type": "invalid_request"},
		})
		return
	}

	pr := chat.ProxyChatCompletion(ctx, w, res.UpstreamBaseURL, apiKey, clientModel, stream, raw, chatTimeout(res), log)
	if pr.Stream {
		return
	}
	if pr.ErrMessage != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(pr.Status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": pr.ErrMessage, "type": "gateway_upstream"},
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(pr.Status)
	_, _ = w.Write(pr.JSONBody)
}

func bearerToken(h string) string {
	h = strings.TrimSpace(h)
	const p = "Bearer "
	if len(h) <= len(p) || !strings.EqualFold(h[:len(p)], p) {
		return ""
	}
	return strings.TrimSpace(h[len(p):])
}

func redactAuth(h string) string {
	h = strings.TrimSpace(h)
	const p = "Bearer "
	if !strings.HasPrefix(strings.ToLower(h), strings.ToLower(p)) {
		return ""
	}
	tok := strings.TrimSpace(h[len(p):])
	if len(tok) <= 8 {
		return "Bearer ***"
	}
	return "Bearer " + tok[:4] + "…"
}

type wrapResponse struct {
	http.ResponseWriter
	status int
}

func (w *wrapResponse) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *wrapResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func healthTimeout(res *config.Resolved) time.Duration {
	return time.Duration(res.HealthTimeoutMs) * time.Millisecond
}

func chatTimeout(res *config.Resolved) time.Duration {
	return time.Duration(res.ChatTimeoutMs) * time.Millisecond
}

func loggingMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wr := &wrapResponse{ResponseWriter: w, status: 200}
		next.ServeHTTP(wr, r)
		if log != nil {
			st := wr.status
			if st == 0 {
				st = 200
			}
			log.Info("http response",
				"method", r.Method,
				"path", r.URL.Path,
				"statusCode", st,
				"responseTimeMs", time.Since(start).Milliseconds(),
				"authorization", redactAuth(r.Header.Get("Authorization")),
			)
		}
	})
}

// ParseLogLevel maps gateway.log_level to slog.Level.
func ParseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// ListenAddrOverride applies -listen flag: "host:port" or ":port".
func ListenAddrOverride(res *config.Resolved, listenFlag string) string {
	if strings.TrimSpace(listenFlag) == "" {
		return res.ListenAddr()
	}
	if strings.HasPrefix(listenFlag, ":") {
		return res.ListenHost + listenFlag
	}
	return listenFlag
}
