package bifrostadmin

import (
	"encoding/json"
	"strings"
)

// ProviderSummary is a small read model for the admin UI (no raw secrets).
type ProviderSummary struct {
	// KeyHint is a masked or descriptive hint for the first API key (e.g. "••••last4", "env:GROQ_API_KEY", "not set").
	KeyHint string `json:"key_hint"`
	// KeyConfigured is true when a non-empty key appears to be configured (direct value or env-backed).
	KeyConfigured bool `json:"key_configured"`
	// OllamaBaseURL is set for the ollama provider from network_config.base_url.
	OllamaBaseURL string `json:"ollama_base_url,omitempty"`
}

// SummarizeProvider parses GET /api/providers/{p} JSON into a ProviderSummary.
// Key "value" may be a string (file / store inline or env.GROQ_API_KEY) or an object (e.g. {"value":"***"} from API).
func SummarizeProvider(providerName string, body []byte) (ProviderSummary, error) {
	var out ProviderSummary
	if len(body) == 0 {
		return out, nil
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return out, err
	}
	if strings.EqualFold(strings.TrimSpace(providerName), "ollama") {
		if nc, ok := root["network_config"].(map[string]any); ok {
			if u, _ := nc["base_url"].(string); strings.TrimSpace(u) != "" {
				out.OllamaBaseURL = strings.TrimSpace(u)
			}
		}
	}
	keys, _ := root["keys"].([]any)
	if len(keys) == 0 {
		out.KeyHint = "not set"
		return out, nil
	}
	k0, ok := keys[0].(map[string]any)
	if !ok {
		out.KeyHint = "not set"
		return out, nil
	}
	out.KeyHint, out.KeyConfigured = summarizeKeyValueField(k0["value"])
	return out, nil
}

func summarizeKeyValueField(raw any) (hint string, configured bool) {
	if raw == nil {
		return "not set", false
	}
	switch v := raw.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return "not set", false
		}
		if strings.HasPrefix(s, "env.") {
			return "env:" + strings.TrimPrefix(s, "env."), true
		}
		if s == "***" || strings.Contains(s, "*") {
			return maskRedactedKey(s), true
		}
		return maskPlainKey(s), true
	case map[string]any:
		fromEnv := false
		switch x := v["from_env"].(type) {
		case bool:
			fromEnv = x
		case float64:
			fromEnv = x != 0
		}
		envVar, _ := v["env_var"].(string)
		if fromEnv && strings.TrimSpace(envVar) != "" {
			return "env:" + strings.TrimSpace(envVar), true
		}
		inner, _ := v["value"].(string)
		inner = strings.TrimSpace(inner)
		if inner == "" || inner == "***" || strings.Contains(inner, "*") {
			if inner != "" {
				return maskRedactedKey(inner), true
			}
			return "not set", false
		}
		return maskPlainKey(inner), true
	default:
		return "not set", false
	}
}

func maskPlainKey(s string) string {
	if len(s) <= 8 {
		return "••••"
	}
	return "••••" + s[len(s)-4:]
}

func maskRedactedKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "configured"
	}
	return s
}
