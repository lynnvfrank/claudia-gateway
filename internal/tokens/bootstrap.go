package tokens

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// IsBootstrapMode reports whether the gateway should run in bootstrap:
// missing tokens file, unreadable, unparseable YAML, or zero valid token rows
// (non-empty token and tenant_id), matching runtime validation in ReloadIfStale.
func IsBootstrapMode(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.IsDir() {
		return true
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	var doc yamlDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return true
	}
	for _, row := range doc.Tokens {
		if strings.TrimSpace(row.Token) != "" && strings.TrimSpace(row.TenantID) != "" {
			return false
		}
	}
	return true
}
