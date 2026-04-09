package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/lynn/claudia-gateway/internal/servicelogs"
)

type logsPollResponse struct {
	Lines  []servicelogs.Entry `json:"lines"`
	MaxSeq uint64              `json:"max_seq"`
}

func (a *adminUI) handleLogsPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store := a.opts.LogStore
	if store == nil {
		http.Error(w, "logs unavailable", http.StatusNotFound)
		return
	}
	var since uint64
	if s := r.URL.Query().Get("since"); s != "" {
		var err error
		since, err = strconv.ParseUint(s, 10, 64)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid since"})
			return
		}
	}
	lines, maxSeq := store.EntriesAfter(since)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(logsPollResponse{Lines: lines, MaxSeq: maxSeq})
}

func (a *adminUI) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	store := a.opts.LogStore
	if store == nil {
		http.Error(w, "logs unavailable", http.StatusNotFound)
		return
	}
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flush := func() { _ = rc.Flush() }
	flush() // prompt clients with headers before replay body

	writeSSE := func(e servicelogs.Entry) {
		b, err := json.Marshal(e)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
	}

	for _, e := range store.Tail(200) {
		writeSSE(e)
	}
	flush()

	ch, cancel := store.Subscribe(64)
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(e)
			flush()
		}
	}
}

func registerUILogs(mux *http.ServeMux, a *adminUI) {
	if a.opts.LogStore == nil {
		return
	}
	mux.HandleFunc("GET /api/ui/logs", a.requireAuthJSON(a.handleLogsPoll))
	mux.HandleFunc("GET /api/ui/logs/stream", a.requireAuthJSON(a.handleLogsStream))
}
