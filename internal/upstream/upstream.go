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

const bifrostCatalogLimit = 500

type bifrostCatalogBody struct {
	Models []struct {
		Name     string `json:"name"`
		Provider string `json:"provider"`
	} `json:"models"`
	Total int `json:"total"`
}

// FetchOpenAIModels prefers BiFrost GET /api/models then falls back to GET /v1/models.
func FetchOpenAIModels(ctx context.Context, baseURL, apiKey string, timeout time.Duration, log *slog.Logger) (status int, body []byte, ok bool) {
	root := strings.TrimSuffix(baseURL, "/")
	catalogURL := fmt.Sprintf("%s/api/models?unfiltered=true&limit=%d", root, bifrostCatalogLimit)
	client := &http.Client{Timeout: timeout}

	if st, b, okCat := tryBifrostCatalog(ctx, client, catalogURL, apiKey, log); okCat {
		return st, b, true
	}

	v1URL := root + "/v1/models"
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
			log.Info("litellm models fetch failed", "err", err, "target", v1URL)
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
			log.Info("litellm models non-OK", "status", res.StatusCode, "target", v1URL)
		}
		return res.StatusCode, b, false
	}
	return res.StatusCode, b, true
}

func tryBifrostCatalog(ctx context.Context, client *http.Client, catalogURL, apiKey string, log *slog.Logger) (status int, body []byte, ok bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogURL, nil)
	if err != nil {
		return 0, nil, false
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	res, err := client.Do(req)
	if err != nil {
		if log != nil {
			log.Debug("upstream catalog fetch failed; falling back to v1/models", "err", err, "target", catalogURL)
		}
		return 0, nil, false
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return 0, nil, false
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if log != nil {
			log.Info("upstream catalog non-OK; falling back to v1/models", "status", res.StatusCode, "target", catalogURL)
		}
		return 0, nil, false
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, nil, false
	}
	var cat bifrostCatalogBody
	if err := json.Unmarshal(b, &cat); err != nil {
		return 0, nil, false
	}
	if len(cat.Models) == 0 {
		return 0, nil, false
	}
	first := cat.Models[0]
	if strings.TrimSpace(first.Name) == "" || strings.TrimSpace(first.Provider) == "" {
		return 0, nil, false
	}
	out, err := bifrostCatalogToOpenAIList(&cat)
	if err != nil {
		return 0, nil, false
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return 0, nil, false
	}
	if log != nil {
		data, _ := out["data"].([]any)
		log.Debug("bifrost catalog models", "route", "GET /api/models (upstream)", "target", catalogURL, "count", len(data))
	}
	return res.StatusCode, encoded, true
}

func bifrostCatalogToOpenAIList(cat *bifrostCatalogBody) (map[string]any, error) {
	now := time.Now().Unix()
	byID := make(map[string]map[string]any)
	for _, m := range cat.Models {
		name := strings.TrimSpace(m.Name)
		prov := strings.TrimSpace(m.Provider)
		if name == "" || prov == "" {
			continue
		}
		id := prov + "/" + name
		if _, exists := byID[id]; exists {
			continue
		}
		byID[id] = map[string]any{
			"id":       id,
			"object":   "model",
			"created":  now,
			"owned_by": prov,
		}
	}
	list := make([]any, 0, len(byID))
	for _, v := range byID {
		list = append(list, v)
	}
	return map[string]any{"object": "list", "data": list}, nil
}

// ProbeHealth performs GET healthURL with optional Bearer (src/litellm.ts probeLitellmHealth).
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
			log.Info("litellm health probe failed", "err", err, "target", healthURL)
		}
		return false, 503, err.Error()
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return false, res.StatusCode, fmt.Sprintf("HTTP %d", res.StatusCode)
	}
	return true, res.StatusCode, ""
}
