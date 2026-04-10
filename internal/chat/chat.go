package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lynn/claudia-gateway/internal/routing"
)

var retryStatuses = map[int]struct{}{
	http.StatusTooManyRequests:     {},
	http.StatusInternalServerError: {},
	http.StatusBadGateway:          {},
	http.StatusServiceUnavailable:  {},
	http.StatusGatewayTimeout:      {},
}

// ProxyResult mirrors src/chat.ts proxyChatCompletion outcomes.
type ProxyResult struct {
	Stream     bool
	Status     int
	JSONBody   []byte
	ErrMessage string
}

// ProxyChatCompletion forwards POST /v1/chat/completions to upstream.
func ProxyChatCompletion(ctx context.Context, w http.ResponseWriter, baseURL, apiKey, upstreamModel string, stream bool, body map[string]json.RawMessage, timeout time.Duration, log *slog.Logger) ProxyResult {
	url := strings.TrimSuffix(baseURL, "/") + "/v1/chat/completions"
	payload := cloneRawMap(body)
	payload["model"] = mustRawJSON(upstreamModel)
	payload["stream"] = mustRawJSON(stream)
	out, err := json.Marshal(payload)
	if err != nil {
		return ProxyResult{Status: 500, ErrMessage: "marshal request"}
	}

	if log != nil {
		log.Debug("upstream chat relay", "upstreamModel", upstreamModel, "stream", stream, "target", url)
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(out))
	if err != nil {
		return ProxyResult{Status: 503, ErrMessage: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		if log != nil {
			log.Info("upstream chat fetch failed", "err", err, "target", url, "upstreamModel", upstreamModel, "stream", stream)
		}
		return ProxyResult{Status: 503, ErrMessage: err.Error()}
	}
	defer res.Body.Close()

	if !statusOK(res.StatusCode) && !stream {
		b, _ := io.ReadAll(res.Body)
		logUpstreamChatResponse(log, url, res.StatusCode, upstreamModel, stream, len(b))
		return ProxyResult{Status: res.StatusCode, JSONBody: b}
	}

	if !statusOK(res.StatusCode) && stream {
		b, err := io.ReadAll(res.Body)
		var wrap []byte
		if err == nil && json.Valid(b) {
			wrap = b
		} else {
			text := string(b)
			if text == "" {
				text = "upstream error on streaming request"
			}
			wrap, _ = json.Marshal(map[string]any{
				"error": map[string]any{
					"message": text,
					"type":    "gateway_upstream",
				},
			})
		}
		logUpstreamChatResponse(log, url, res.StatusCode, upstreamModel, stream, len(wrap))
		return ProxyResult{Status: res.StatusCode, JSONBody: wrap}
	}

	if stream && res.Body != nil {
		h := w.Header()
		ct := res.Header.Get("Content-Type")
		if ct == "" {
			ct = "text/event-stream; charset=utf-8"
		}
		h.Set("Content-Type", ct)
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "keep-alive")
		if x := res.Header.Get("X-Request-Id"); x != "" {
			h.Set("X-Request-Id", x)
		}
		w.WriteHeader(http.StatusOK)
		var cw countWriter
		cw.w = w
		if f, ok := w.(http.Flusher); ok {
			_, _ = io.Copy(&flushWriter{w: &cw, f: f}, res.Body)
		} else {
			_, _ = io.Copy(&cw, res.Body)
		}
		logUpstreamChatResponse(log, url, http.StatusOK, upstreamModel, stream, int(cw.n))
		return ProxyResult{Stream: true}
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		logUpstreamChatResponse(log, url, res.StatusCode, upstreamModel, stream, 0)
		return ProxyResult{Status: 503, ErrMessage: err.Error()}
	}
	logUpstreamChatResponse(log, url, res.StatusCode, upstreamModel, stream, len(b))
	return ProxyResult{Status: res.StatusCode, JSONBody: b}
}

// countWriter wraps an io.Writer and records the number of bytes written.
type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func logUpstreamChatResponse(log *slog.Logger, url string, status int, upstreamModel string, stream bool, responseBytes int) {
	if log == nil {
		return
	}
	log.Info("upstream chat response",
		"route", "POST /v1/chat/completions (upstream)",
		"target", url,
		"status", status,
		"upstreamModel", upstreamModel,
		"stream", stream,
		"responseBytes", responseBytes,
	)
}

type flushWriter struct {
	w io.Writer
	f http.Flusher
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if n > 0 {
		fw.f.Flush()
	}
	return n, err
}

func statusOK(code int) bool {
	return code >= 200 && code < 300
}

func cloneRawMap(m map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(m)+2)
	for k, v := range m {
		out[k] = v
	}
	return out
}

func mustRawJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

// WithVirtualModelFallback implements src/chat.ts chatWithVirtualModelFallback.
func WithVirtualModelFallback(ctx context.Context, w http.ResponseWriter, initialUpstream string, fallbackChain []string, baseURL, apiKey string, stream bool, body map[string]json.RawMessage, timeout time.Duration, log *slog.Logger) {
	start := routing.StartingFallbackIndex(initialUpstream, fallbackChain)
	var chain []string
	if len(fallbackChain) > 0 {
		chain = fallbackChain[start:]
	} else if initialUpstream != "" {
		chain = []string{initialUpstream}
	}

	if len(chain) == 0 {
		writeJSONError(w, http.StatusServiceUnavailable, map[string]any{
			"message": "No upstream models configured (routing.fallback_chain empty and no initial model).",
			"type":    "gateway_config",
		})
		return
	}

	var lastNonRetry *struct {
		status int
		body   []byte
	}

	for i, upstreamModel := range chain {
		if log != nil {
			log.Debug("virtual model fallback attempt", "attempt", i+1, "upstreamModel", upstreamModel, "chainLen", len(chain))
		}
		r := ProxyChatCompletion(ctx, w, baseURL, apiKey, upstreamModel, stream, body, timeout, log)
		if r.Stream {
			return
		}
		if r.ErrMessage != "" {
			if _, retry := retryStatuses[r.Status]; retry && i < len(chain)-1 {
				continue
			}
			writeJSONError(w, r.Status, map[string]any{"message": r.ErrMessage, "type": "gateway_upstream"})
			return
		}
		if r.JSONBody != nil {
			if _, retry := retryStatuses[r.Status]; retry && i < len(chain)-1 {
				if log != nil {
					log.Info("retrying next fallback model", "upstreamModel", upstreamModel, "status", r.Status, "willRetry", true)
				}
				continue
			}
			if _, retry := retryStatuses[r.Status]; !retry {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(r.Status)
				_, _ = w.Write(r.JSONBody)
				return
			}
			lastNonRetry = &struct {
				status int
				body   []byte
			}{r.Status, r.JSONBody}
		}
	}

	if lastNonRetry != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(lastNonRetry.status)
		_, _ = w.Write(lastNonRetry.body)
		return
	}

	writeJSONError(w, http.StatusServiceUnavailable, map[string]any{
		"message": "Exhausted fallback chain without a successful completion.",
		"type":    "gateway_exhausted",
	})
}

func writeJSONError(w http.ResponseWriter, code int, errObj map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": errObj})
}
