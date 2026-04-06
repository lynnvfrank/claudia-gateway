package routing

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// Via matches TypeScript pickInitialModel return.
type Via string

const (
	ViaRule             Via = "rule"
	ViaAmbiguousDefault Via = "ambiguous_default"
	ViaChainOnly        Via = "chain_only"
)

type policyDoc struct {
	AmbiguousDefault string `yaml:"ambiguous_default_model"`
	Rules            []struct {
		Name string `yaml:"name"`
		When struct {
			MinMessageChars *int `yaml:"min_message_chars"`
		} `yaml:"when"`
		Models []string `yaml:"models"`
	} `yaml:"rules"`
}

// Policy reloads routing-policy.yaml on mtime (src/routing.ts).
type Policy struct {
	path             string
	log              *slog.Logger
	mu               sync.Mutex
	mtimeNs          int64
	ambiguousDefault string
	rules            []policyRule
}

type policyRule struct {
	name            string
	minMessageChars *int
	models          []string
}

func NewPolicy(path string, log *slog.Logger) *Policy {
	return &Policy{path: path, log: log}
}

func (p *Policy) ReloadIfStale() {
	p.mu.Lock()
	defer p.mu.Unlock()

	st, err := os.Stat(p.path)
	if err != nil {
		if p.log != nil {
			p.log.Error("routing policy file missing", "path", p.path, "err", err)
		}
		p.rules = nil
		p.ambiguousDefault = ""
		p.mtimeNs = 0
		return
	}
	mt := st.ModTime().UnixNano()
	if mt == p.mtimeNs {
		return
	}
	p.mtimeNs = mt

	raw, err := os.ReadFile(p.path)
	if err != nil {
		if p.log != nil {
			p.log.Error("read routing policy", "path", p.path, "err", err)
		}
		return
	}
	var doc policyDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		if p.log != nil {
			p.log.Error("failed to parse routing policy yaml", "path", p.path, "err", err)
		}
		return
	}
	p.ambiguousDefault = doc.AmbiguousDefault
	var rules []policyRule
	for _, r := range doc.Rules {
		if len(r.Models) == 0 {
			continue
		}
		rules = append(rules, policyRule{
			name:            r.Name,
			minMessageChars: r.When.MinMessageChars,
			models:          r.Models,
		})
	}
	p.rules = rules
	if p.log != nil {
		p.log.Info("reloaded routing policy", "path", p.path, "rules", len(p.rules))
	}
}

// PickInitialModel returns the first upstream model id for the virtual Claudia model.
func (p *Policy) PickInitialModel(body map[string]json.RawMessage, fallbackChain []string, virtualModelID string) (model string, via Via) {
	p.ReloadIfStale()

	var clientModel string
	if m, ok := body["model"]; ok {
		_ = json.Unmarshal(m, &clientModel)
	}
	if clientModel != virtualModelID {
		return clientModel, ViaChainOnly
	}

	lastUser := lastUserMessageCharCount(body)

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, rule := range p.rules {
		if len(rule.models) == 0 {
			continue
		}
		if rule.minMessageChars != nil && lastUser < *rule.minMessageChars {
			continue
		}
		if p.log != nil {
			p.log.Debug("routing rule matched", "rule", rule.name, "initialModel", rule.models[0], "lastUserChars", lastUser)
		}
		return rule.models[0], ViaRule
	}

	if p.ambiguousDefault != "" {
		if p.log != nil {
			p.log.Debug("routing: no rule matched, using ambiguous_default_model", "initialModel", p.ambiguousDefault, "lastUserChars", lastUser)
		}
		return p.ambiguousDefault, ViaAmbiguousDefault
	}

	first := ""
	if len(fallbackChain) > 0 {
		first = fallbackChain[0]
	}
	if p.log != nil {
		p.log.Debug("routing: no policy default; using first fallback_chain entry", "initialModel", first, "lastUserChars", lastUser)
	}
	return first, ViaChainOnly
}

func lastUserMessageCharCount(body map[string]json.RawMessage) int {
	raw, ok := body["messages"]
	if !ok {
		return 0
	}
	var messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &messages); err != nil {
		return 0
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		return contentLength(messages[i].Content)
	}
	return 0
}

func contentLength(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return 0
		}
		return len([]rune(s))
	}
	if raw[0] == '[' {
		var parts []struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &parts); err != nil {
			return 0
		}
		n := 0
		for _, part := range parts {
			n += len([]rune(part.Text))
		}
		return n
	}
	return 0
}

// StartingFallbackIndex is src/routing.ts startingFallbackIndex.
func StartingFallbackIndex(initialModel string, fallbackChain []string) int {
	for i, m := range fallbackChain {
		if m == initialModel {
			return i
		}
	}
	return 0
}
