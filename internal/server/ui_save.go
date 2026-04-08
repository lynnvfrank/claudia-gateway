package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/lynn/claudia-gateway/internal/bifrostadmin"
)

const maxProviderErrorBody = 2048

func truncateErrMsg(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxProviderErrorBody {
		return s
	}
	return s[:maxProviderErrorBody] + "…"
}

func (a *adminUI) saveKeyHandler(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Value string `json:"value"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		if err := dec.Decode(&body); err != nil {
			writeUIJSONError(w, http.StatusBadRequest, "invalid json", "")
			return
		}
		v := strings.TrimSpace(body.Value)
		if v == "" {
			writeUIJSONError(w, http.StatusBadRequest, "value required", "")
			return
		}
		ctx := r.Context()
		client := bifrostAdminClient(a.rt)
		cur, st, err := client.GetProvider(ctx, provider)
		if err != nil {
			writeUIJSONError(w, http.StatusBadGateway, "bifrost unreachable", truncateErrMsg(err.Error()))
			return
		}
		cur, ok := bifrostadmin.NormalizeProviderGETForMerge(st, cur)
		if !ok {
			writeUIJSONError(w, http.StatusBadGateway, fmt.Sprintf("bifrost GET %d", st), truncateErrMsg(string(cur)))
			return
		}
		merged, err := bifrostadmin.MergeProviderKey(provider, cur, v)
		if err != nil {
			writeUIJSONError(w, http.StatusInternalServerError, "merge failed", truncateErrMsg(err.Error()))
			return
		}
		pst, pbody, err := client.PutProvider(ctx, provider, merged)
		if err != nil {
			writeUIJSONError(w, http.StatusBadGateway, "bifrost PUT failed", truncateErrMsg(err.Error()))
			return
		}
		if pst < 200 || pst >= 300 {
			writeUIJSONError(w, http.StatusBadGateway, fmt.Sprintf("bifrost PUT %d", pst), truncateErrMsg(string(pbody)))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}
}

func (a *adminUI) saveOllamaBaseURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		BaseURL string `json:"base_url"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		writeUIJSONError(w, http.StatusBadRequest, "invalid json", "")
		return
	}
	u := strings.TrimSpace(body.BaseURL)
	if u == "" {
		writeUIJSONError(w, http.StatusBadRequest, "base_url required", "")
		return
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		writeUIJSONError(w, http.StatusBadRequest, "invalid base_url", "")
		return
	}
	ctx := r.Context()
	client := bifrostAdminClient(a.rt)
	cur, st, err := client.GetProvider(ctx, "ollama")
	if err != nil {
		writeUIJSONError(w, http.StatusBadGateway, "bifrost unreachable", truncateErrMsg(err.Error()))
		return
	}
	cur, ok := bifrostadmin.NormalizeProviderGETForMerge(st, cur)
	if !ok {
		writeUIJSONError(w, http.StatusBadGateway, fmt.Sprintf("bifrost GET %d", st), truncateErrMsg(string(cur)))
		return
	}
	merged, err := bifrostadmin.MergeOllamaBaseURL(cur, u)
	if err != nil {
		writeUIJSONError(w, http.StatusInternalServerError, "merge failed", truncateErrMsg(err.Error()))
		return
	}
	pst, pbody, err := client.PutProvider(ctx, "ollama", merged)
	if err != nil {
		writeUIJSONError(w, http.StatusBadGateway, "bifrost PUT failed", truncateErrMsg(err.Error()))
		return
	}
	if pst < 200 || pst >= 300 {
		writeUIJSONError(w, http.StatusBadGateway, fmt.Sprintf("bifrost PUT %d", pst), truncateErrMsg(string(pbody)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func writeUIJSONError(w http.ResponseWriter, code int, msg, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":  msg,
		"detail": detail,
	})
}

func (a *adminUI) handleLogoutPOST(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if c, err := r.Cookie(a.cookieName()); err == nil && c.Value != "" {
		a.opts.Sessions.revoke(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   a.cookieName(),
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	w.WriteHeader(http.StatusNoContent)
}
