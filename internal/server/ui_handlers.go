package server

import (
	"context"
	"embed"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lynn/claudia-gateway/internal/bifrostadmin"
)

//go:embed embedui/login.html embedui/panel.html
var adminEmbedUI embed.FS

func bifrostAdminClient(rt *Runtime) *bifrostadmin.Client {
	rt.Sync()
	res, _, _ := rt.Snapshot()
	if res == nil {
		return &bifrostadmin.Client{}
	}
	tok := ""
	if res.UpstreamAPIKeyEnv != "" {
		tok = strings.TrimSpace(os.Getenv(res.UpstreamAPIKeyEnv))
	}
	return &bifrostadmin.Client{
		BaseURL:     res.UpstreamBaseURL,
		BearerToken: tok,
		HTTPClient:  &http.Client{Timeout: 8 * time.Second},
	}
}

func publicGatewayBase(r *http.Request) string {
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return "http://127.0.0.1:3000"
	}
	return "http://" + host
}

type adminUI struct {
	rt   *Runtime
	log  *slog.Logger
	opts *UIOptions
}

func (a *adminUI) cookieName() string { return a.opts.cookieName() }

func (a *adminUI) sessionOK(r *http.Request) bool {
	c, err := r.Cookie(a.cookieName())
	if err != nil || c.Value == "" {
		return false
	}
	return a.opts.Sessions.valid(c.Value)
}

func (a *adminUI) requireAuthJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.sessionOK(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (a *adminUI) requireAuthPage(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.sessionOK(r) {
			http.Redirect(w, r, "/ui/login", http.StatusFound)
			return
		}
		next(w, r)
	}
}

func (a *adminUI) serveEmbed(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := adminEmbedUI.ReadFile(name)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	}
}

func (a *adminUI) handleLoginPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid json"})
		return
	}
	token := strings.TrimSpace(body.Token)
	a.rt.Sync()
	_, tokStore, _ := a.rt.Snapshot()
	if tokStore == nil || tokStore.Validate(token) == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid token"})
		return
	}
	sid, err := a.opts.Sessions.issue(token)
	if err != nil {
		if a.log != nil {
			a.log.Error("ui session issue", "err", err)
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     a.cookieName(),
		Value:    sid,
		Path:     "/",
		MaxAge:   int((24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (a *adminUI) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.rt.Sync()
	res, _, _ := a.rt.Snapshot()
	if res == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "gateway not configured"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	client := bifrostAdminClient(a.rt)
	providers := []string{"groq", "gemini", "ollama"}
	provOut := make(map[string]any, len(providers))
	for _, name := range providers {
		b, st, err := client.GetProvider(ctx, name)
		entry := map[string]any{"provider": name}
		if err != nil {
			entry["ok"] = false
			entry["error"] = err.Error()
			provOut[name] = entry
			continue
		}
		if bifrostadmin.IsProviderMissingGET(st, b) {
			entry["ok"] = true
			entry["key_configured"] = false
			entry["key_hint"] = ""
			if name == "ollama" {
				entry["ollama_base_url"] = ""
			}
			provOut[name] = entry
			continue
		}
		entry["http_status"] = st
		if st < 200 || st >= 300 {
			entry["ok"] = false
			entry["error"] = strings.TrimSpace(string(b))
			if entry["error"] == "" {
				entry["error"] = http.StatusText(st)
			}
			provOut[name] = entry
			continue
		}
		sum, serr := bifrostadmin.SummarizeProvider(name, b)
		if serr != nil {
			entry["ok"] = false
			entry["error"] = serr.Error()
			provOut[name] = entry
			continue
		}
		entry["ok"] = true
		entry["key_hint"] = sum.KeyHint
		entry["key_configured"] = sum.KeyConfigured
		if sum.OllamaBaseURL != "" {
			entry["ollama_base_url"] = sum.OllamaBaseURL
		}
		provOut[name] = entry
	}
	gwOut := map[string]any{
		"semver":           res.Semver,
		"virtual_model_id": res.VirtualModelID,
		"public_base_url":  publicGatewayBase(r),
		"token_hint":       "Paste the same gateway token you used to sign in (stored only in Continue on your machine).",
	}
	if c, err := r.Cookie(a.cookieName()); err == nil && c.Value != "" {
		if tok := a.opts.Sessions.GatewayToken(c.Value); tok != "" {
			gwOut["continue_gateway_token"] = tok
		}
	}
	out := map[string]any{
		"gateway":   gwOut,
		"providers": provOut,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func registerAdminUI(mux *http.ServeMux, rt *Runtime, log *slog.Logger, ui *UIOptions) {
	if ui == nil || ui.Sessions == nil {
		return
	}
	a := &adminUI{rt: rt, log: log, opts: ui}

	mux.HandleFunc("GET /ui", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if a.sessionOK(r) {
			http.Redirect(w, r, "/ui/panel", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/ui/login", http.StatusFound)
	})

	mux.HandleFunc("GET /ui/login", a.serveEmbed("embedui/login.html"))
	mux.HandleFunc("GET /ui/panel", a.requireAuthPage(a.serveEmbed("embedui/panel.html")))

	mux.HandleFunc("POST /api/ui/login", a.handleLoginPOST)
	mux.HandleFunc("POST /api/ui/logout", a.handleLogoutPOST)
	mux.HandleFunc("GET /api/ui/state", a.requireAuthJSON(a.handleState))

	mux.HandleFunc("POST /api/ui/provider/groq/key", a.requireAuthJSON(a.saveKeyHandler("groq")))
	mux.HandleFunc("POST /api/ui/provider/gemini/key", a.requireAuthJSON(a.saveKeyHandler("gemini")))
	mux.HandleFunc("POST /api/ui/provider/ollama/base_url", a.requireAuthJSON(a.saveOllamaBaseURL))
}
