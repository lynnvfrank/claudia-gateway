package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// FetchOpenAIModels fetches GET /v1/models from the upstream (OpenAI-compatible model list).
func FetchOpenAIModels(ctx context.Context, baseURL, apiKey string, timeout time.Duration, log *slog.Logger) (status int, body []byte, ok bool) {
	root := strings.TrimSuffix(baseURL, "/")
	v1URL := root + "/v1/models"
	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v1URL, nil)
	if err != nil {
		return 0, nil, false
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	res, err := client.Do(req)
	if err != nil {
		if log != nil {
			log.Info("upstream models fetch failed", "err", err, "target", v1URL)
		}
		return 503, nil, false
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, nil, false
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if log != nil {
			log.Info("upstream models non-OK", "status", res.StatusCode, "target", v1URL)
		}
		return res.StatusCode, b, false
	}
	if log != nil {
		var wrap struct {
			Data []any `json:"data"`
		}
		if json.Unmarshal(b, &wrap) == nil {
			log.Debug("upstream models", "route", "GET /v1/models", "target", v1URL, "count", len(wrap.Data))
		}
	}
	return res.StatusCode, b, true
}

// ProbeHealth performs GET healthURL with optional Bearer token.
func ProbeHealth(ctx context.Context, healthURL, apiKey string, timeout time.Duration, log *slog.Logger) (ok bool, status int, detail string) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return false, 500, err.Error()
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	res, err := client.Do(req)
	if err != nil {
		if log != nil {
			log.Info("upstream health probe failed", "err", err, "target", healthURL)
		}
		return false, 503, err.Error()
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return false, res.StatusCode, fmt.Sprintf("HTTP %d", res.StatusCode)
	}
	return true, res.StatusCode, ""
}
