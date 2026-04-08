package bifrostadmin

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MergeProviderKey returns JSON suitable for PUT /api/providers/{provider} by copying the
// current GET response and setting the first key's inline value (or adding a minimal key row).
// provider must match the BiFrost provider id (e.g. groq, gemini); new keys use a name unique
// per provider so the config store does not reject duplicates across providers.
func MergeProviderKey(provider string, existingJSON []byte, plaintextKey string) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(existingJSON, &root); err != nil {
		return nil, err
	}
	ensureConcurrency(root)
	keys, ok := root["keys"].([]any)
	if !ok || len(keys) == 0 {
		root["keys"] = []any{minimalKeyRow(provider, plaintextKey)}
	} else {
		k0, ok := keys[0].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("bifrostadmin: keys[0] is not an object")
		}
		// Inline secret as a string — matches bifrost.config.json / store shape (nested {"value":{...}} is mis-stored as JSON text).
		k0["value"] = plaintextKey
		keys[0] = k0
		root["keys"] = keys
	}
	return json.Marshal(root)
}

// MergeOllamaBaseURL updates network_config.base_url while preserving other provider fields.
func MergeOllamaBaseURL(existingJSON []byte, baseURL string) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(existingJSON, &root); err != nil {
		return nil, err
	}
	ensureConcurrency(root)
	if _, ok := root["keys"]; !ok {
		root["keys"] = []any{}
	}
	nc, ok := root["network_config"].(map[string]any)
	if !ok {
		nc = map[string]any{}
	}
	nc["base_url"] = baseURL
	root["network_config"] = nc
	return json.Marshal(root)
}

func ensureConcurrency(root map[string]any) {
	if root == nil {
		return
	}
	if _, ok := root["concurrency_and_buffer_size"]; !ok {
		root["concurrency_and_buffer_size"] = map[string]any{
			"concurrency": float64(100),
			"buffer_size": float64(200),
		}
	}
}

func minimalKeyRow(provider, plaintext string) map[string]any {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		p = "unknown"
	}
	return map[string]any{
		"name":    "claudia-ui-" + p,
		"weight":  float64(1),
		"enabled": true,
		"value":   plaintext,
	}
}
