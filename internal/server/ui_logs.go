package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/lynn/claudia-gateway/internal/servicelogs"
)

type logsPollResponse struct {
	Lines         []servicelogs.Entry `json:"lines"`
	MaxSeq        uint64              `json:"max_seq"`
	BufferMinSeq  uint64              `json:"buffer_min_seq,omitempty"`
	HasOlderInBuf *bool               `json:"has_older_in_buffer,omitempty"`
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
	hasSince := r.URL.Query().Get("since") != ""
	hasBefore := r.URL.Query().Get("before_seq") != ""
	if hasSince && hasBefore {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "use either since or before_seq, not both"})
		return
	}

	var limit int
	if ls := r.URL.Query().Get("limit"); ls != "" {
		v, err := strconv.Atoi(ls)
		if err != nil || v < 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid limit"})
			return
		}
		limit = v
		if limit > servicelogs.DefaultMaxLines {
			limit = servicelogs.DefaultMaxLines
		}
	}

	bufMin := store.MinSeq()
	resp := logsPollResponse{BufferMinSeq: bufMin}

	if hasBefore {
		bs := r.URL.Query().Get("before_seq")
		beforeSeq, err := strconv.ParseUint(bs, 10, 64)
		if err != nil || beforeSeq <= 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid before_seq"})
			return
		}
		if limit <= 0 {
			limit = 300
		}
		lines := store.EntriesBefore(beforeSeq, limit)
		resp.Lines = lines
		_, resp.MaxSeq = store.EntriesAfter(0)
		if len(lines) > 0 {
			v := lines[0].Seq > bufMin
			resp.HasOlderInBuf = &v
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
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
	if limit > 0 && len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	resp.Lines = lines
	resp.MaxSeq = maxSeq
	if since == 0 && len(lines) > 0 {
		v := lines[0].Seq > bufMin
		resp.HasOlderInBuf = &v
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
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
