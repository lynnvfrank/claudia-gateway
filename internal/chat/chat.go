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

	"github.com/lynn/claudia-gateway/internal/gatewaymetrics"
	"github.com/lynn/claudia-gateway/internal/providerlimits"
	"github.com/lynn/claudia-gateway/internal/routing"
	"github.com/lynn/claudia-gateway/internal/tokencount"
)

var retryStatuses = map[int]struct{}{
	http.StatusRequestEntityTooLarge: {}, // 413: virtual model tries next fallback (same payload)
	http.StatusTooManyRequests:       {},
	http.StatusInternalServerError:   {},
	http.StatusBadGateway:            {},
	http.StatusServiceUnavailable:    {},
	http.StatusGatewayTimeout:        {},
}

// hasMoreFallbackCandidates reports whether any chain entry after afterIdx is not excluded for 413
// (and thus is eligible to try). excluded413 may be nil.
func hasMoreFallbackCandidates(chain []string, afterIdx int, excluded413 map[string]struct{}) bool {
	for j := afterIdx + 1; j < len(chain); j++ {
		if excluded413 != nil {
			if _, skip := excluded413[chain[j]]; skip {
				continue
			}
		}
		return true
	}
	return false
}

// ProxyResult mirrors src/chat.ts proxyChatCompletion outcomes.
type ProxyResult struct {
	Stream     bool
	Status     int
	JSONBody   []byte
	ErrMessage string
}

func estTokensFromPayload(out []byte) int {
	n, err := tokencount.Count(string(out))
	if err != nil {
		return 0
	}
	return n
}

// prepareChatPayload builds the proxied JSON body and its estimated token count for upstreamModel.
func prepareChatPayload(upstreamModel string, stream bool, body map[string]json.RawMessage) ([]byte, int, error) {
	payload := cloneRawMap(body)
	payload["model"] = mustRawJSON(upstreamModel)
	payload["stream"] = mustRawJSON(stream)
	out, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	return out, estTokensFromPayload(out), nil
}

func recordUpstreamMetrics(rec gatewaymetrics.Recorder, upstreamModel string, status, est int) {
	if rec == nil {
		return
	}
	rec.RecordUpstreamResponse(time.Now().UTC(), upstreamModel, status, est)
}

// ProxyChatCompletion forwards POST /v1/chat/completions to upstream. When guard is non-nil,
// admission is checked before the HTTP request; denial returns HTTP 429 with JSON in JSONBody.
func ProxyChatCompletion(ctx context.Context, w http.ResponseWriter, baseURL, apiKey, upstreamModel string, stream bool, body map[string]json.RawMessage, timeout time.Duration, log *slog.Logger, rec gatewaymetrics.Recorder, guard *providerlimits.Guard) ProxyResult {
	out, est, err := prepareChatPayload(upstreamModel, stream, body)
	if err != nil {
		return ProxyResult{Status: 500, ErrMessage: "marshal request"}
	}
	if guard != nil {
		d, gerr := guard.Allow(ctx, upstreamModel, int64(est))
		if gerr != nil && log != nil {
			log.Warn("provider limits admission query failed; allowing request", "err", gerr, "upstreamModel", upstreamModel)
		}
		if !d.Allowed {
			if log != nil {
				log.Info("chat blocked by provider limits", "upstreamModel", upstreamModel, "reason", d.Reason, "detail", d.Detail)
			}
			b, _ := json.Marshal(map[string]any{
				"error": map[string]any{
					"message": "Configured provider/model quota would be exceeded for this request (" + string(d.Reason) + ").",
					"type":    "gateway_provider_limits",
				},
			})
			return ProxyResult{Status: http.StatusTooManyRequests, JSONBody: b}
		}
	}
	return proxyChatCompletionPayload(ctx, w, baseURL, apiKey, upstreamModel, stream, out, est, timeout, log, rec)
}

func proxyChatCompletionPayload(ctx context.Context, w http.ResponseWriter, baseURL, apiKey, upstreamModel string, stream bool, out []byte, est int, timeout time.Duration, log *slog.Logger, rec gatewaymetrics.Recorder) ProxyResult {
	url := strings.TrimSuffix(baseURL, "/") + "/v1/chat/completions"

	if log != nil {
		n, errTok := tokencount.Count(string(out))
		if errTok == nil {
			log.Info("upstream chat relay",
				"msg", "chat.bifrost.request",
				"upstreamModel", upstreamModel,
				"stream", stream,
				"target", url,
				"outgoingTokens", n,
			)
		} else {
			log.Info("upstream chat relay",
				"msg", "chat.bifrost.request",
				"upstreamModel", upstreamModel,
				"stream", stream,
				"target", url,
			)
			log.Debug("outgoing token count failed", "err", errTok)
		}
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
			log.Info("upstream chat fetch failed", "msg", "chat.bifrost.error", "err", err, "target", url, "upstreamModel", upstreamModel, "stream", stream)
		}
		recordUpstreamMetrics(rec, upstreamModel, http.StatusServiceUnavailable, est)
		return ProxyResult{Status: 503, ErrMessage: err.Error()}
	}
	defer res.Body.Close()

	if !statusOK(res.StatusCode) && !stream {
		b, _ := io.ReadAll(res.Body)
		logUpstreamChatResponse(log, url, res.StatusCode, upstreamModel, stream, len(b))
		recordUpstreamMetrics(rec, upstreamModel, res.StatusCode, est)
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
		recordUpstreamMetrics(rec, upstreamModel, res.StatusCode, est)
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
		recordUpstreamMetrics(rec, upstreamModel, http.StatusOK, est)
		return ProxyResult{Stream: true}
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		logUpstreamChatResponse(log, url, res.StatusCode, upstreamModel, stream, 0)
		recordUpstreamMetrics(rec, upstreamModel, http.StatusServiceUnavailable, est)
		return ProxyResult{Status: 503, ErrMessage: err.Error()}
	}
	logUpstreamChatResponse(log, url, res.StatusCode, upstreamModel, stream, len(b))
	recordUpstreamMetrics(rec, upstreamModel, res.StatusCode, est)
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
func WithVirtualModelFallback(ctx context.Context, w http.ResponseWriter, initialUpstream string, fallbackChain []string, baseURL, apiKey string, stream bool, body map[string]json.RawMessage, timeout time.Duration, log *slog.Logger, rec gatewaymetrics.Recorder, guard *providerlimits.Guard) {
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
	// Upstream ids that returned HTTP 413 on this request are not tried again (duplicate ids in chain).
	excluded413 := make(map[string]struct{})

	for i, upstreamModel := range chain {
		if _, skip := excluded413[upstreamModel]; skip {
			if log != nil {
				log.Debug("virtual model skipping model (413 earlier this request)", "upstreamModel", upstreamModel, "index", i)
			}
			continue
		}
		if log != nil {
			log.Debug("virtual model fallback attempt", "attempt", i+1, "upstreamModel", upstreamModel, "chainLen", len(chain))
		}
		out, est, err := prepareChatPayload(upstreamModel, stream, body)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, map[string]any{"message": "marshal request", "type": "gateway_internal"})
			return
		}
		if guard != nil {
			d, gerr := guard.Allow(ctx, upstreamModel, int64(est))
			if gerr != nil && log != nil {
				log.Warn("provider limits admission query failed; allowing attempt", "err", gerr, "upstreamModel", upstreamModel)
			}
			if !d.Allowed {
				if log != nil {
					log.Info("skipping upstream model (provider limits)", "upstreamModel", upstreamModel, "reason", d.Reason, "detail", d.Detail)
				}
				if i < len(chain)-1 {
					continue
				}
				writeJSONError(w, http.StatusTooManyRequests, map[string]any{
					"message": "Every model in the fallback chain would exceed configured provider quotas (" + string(d.Reason) + ").",
					"type":    "gateway_provider_limits",
				})
				return
			}
		}
		r := proxyChatCompletionPayload(ctx, w, baseURL, apiKey, upstreamModel, stream, out, est, timeout, log, rec)
		if r.Status == http.StatusRequestEntityTooLarge {
			excluded413[upstreamModel] = struct{}{}
		}
		if r.Stream {
			return
		}
		if r.ErrMessage != "" {
			if _, retry := retryStatuses[r.Status]; retry && hasMoreFallbackCandidates(chain, i, excluded413) {
				continue
			}
			writeJSONError(w, r.Status, map[string]any{"message": r.ErrMessage, "type": "gateway_upstream"})
			return
		}
		if r.JSONBody != nil {
			if _, retry := retryStatuses[r.Status]; retry && hasMoreFallbackCandidates(chain, i, excluded413) {
				if log != nil {
					log.Info("retrying next fallback model", "msg", "chat.routing.fallback", "upstreamModel", upstreamModel, "status", r.Status, "willRetry", true)
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
