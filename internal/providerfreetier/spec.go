package providerfreetier

import (
	"fmt"
	"os"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// Spec is an operator-maintained allowlist of upstream model ids (BiFrost style: provider/model).
type Spec struct {
	FormatVersion int      `yaml:"format_version"`
	EffectiveDate string   `yaml:"effective_date"`
	Models        []string `yaml:"models"`
	Patterns      []string `yaml:"patterns"`
}

// Load reads and parses provider-free-tier YAML. An empty or missing models/patterns list is valid.
func Load(filePath string) (*Spec, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var s Spec
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parse provider free tier yaml: %w", err)
	}
	if s.FormatVersion != 1 {
		return nil, fmt.Errorf("provider free tier: unsupported format_version %d (supported: 1)", s.FormatVersion)
	}
	return &s, nil
}

// Empty reports whether there are no exact entries and no patterns.
func (s *Spec) Empty() bool {
	if s == nil {
		return true
	}
	return len(s.Models) == 0 && len(s.Patterns) == 0
}

// Match returns true if id is allowed by exact list or shell-style patterns (* matches any segment within one path element for path.Match).
func (s *Spec) Match(id string) bool {
	if s == nil {
		return false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, m := range s.Models {
		if strings.TrimSpace(m) == id {
			return true
		}
	}
	for _, p := range s.Patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		ok, err := path.Match(p, id)
		if err == nil && ok {
			return true
		}
	}
	return false
}

// Filter returns ids that match the spec, preserving input order.
func (s *Spec) Filter(ids []string) []string {
	if s == nil {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || !s.Match(id) {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
