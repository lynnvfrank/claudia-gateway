package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Resolved matches TypeScript ResolvedGatewayConfig (src/config.ts).
type Resolved struct {
	Semver            string
	VirtualModelID    string
	ListenPort        int
	ListenHost        string
	LogLevel          string
	LitellmBaseURL    string
	LitellmAPIKeyEnv  string
	HealthLitellmURL  string
	HealthTimeoutMs   int
	ChatTimeoutMs     int
	TokensPath        string
	RoutingPolicyPath string
	FallbackChain     []string
	GatewayYAMLPath   string
}

type gatewayDoc struct {
	Gateway struct {
		Semver     string `yaml:"semver"`
		ListenPort int    `yaml:"listen_port"`
		ListenHost string `yaml:"listen_host"`
		LogLevel   string `yaml:"log_level"`
	} `yaml:"gateway"`
	Litellm struct {
		BaseURL   string `yaml:"base_url"`
		APIKeyEnv string `yaml:"api_key_env"`
	} `yaml:"litellm"`
	Health struct {
		LitellmURL string `yaml:"litellm_url"`
		TimeoutMs  int    `yaml:"timeout_ms"`
		ChatMs     int    `yaml:"chat_timeout_ms"`
	} `yaml:"health"`
	Paths struct {
		Tokens        string `yaml:"tokens"`
		RoutingPolicy string `yaml:"routing_policy"`
	} `yaml:"paths"`
	Routing struct {
		FallbackChain []string `yaml:"fallback_chain"`
	} `yaml:"routing"`
}

const (
	defaultSemver          = "0.1.0"
	defaultListenPort      = 3000
	defaultListenHost      = "0.0.0.0"
	defaultLogLevel        = "info"
	defaultBaseURL         = "http://bifrost:8080"
	defaultAPIKeyEnv       = "CLAUDIA_UPSTREAM_API_KEY"
	defaultHealthTimeoutMs = 5000
	defaultChatTimeoutMs   = 300_000
)

// LoadGatewayYAML reads and parses gateway.yaml at filePath (absolute or cwd-relative).
func LoadGatewayYAML(filePath string, log *slog.Logger) (*Resolved, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var doc gatewayDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse gateway yaml: %w", err)
	}

	semver := doc.Gateway.Semver
	if semver == "" {
		semver = defaultSemver
	}
	litellmBase := strings.TrimSuffix(doc.Litellm.BaseURL, "/")
	if litellmBase == "" {
		litellmBase = strings.TrimSuffix(defaultBaseURL, "/")
	}
	apiKeyEnv := doc.Litellm.APIKeyEnv
	if apiKeyEnv == "" {
		apiKeyEnv = defaultAPIKeyEnv
	}

	healthURL := strings.TrimSpace(doc.Health.LitellmURL)
	if healthURL == "" {
		healthURL = litellmBase + "/health"
	}

	baseDir := filepath.Dir(filePath)
	tokensRel := doc.Paths.Tokens
	if tokensRel == "" {
		tokensRel = "./tokens.yaml"
	}
	routeRel := doc.Paths.RoutingPolicy
	if routeRel == "" {
		routeRel = "./routing-policy.yaml"
	}
	tokensPath := filepath.Join(baseDir, tokensRel)
	if filepath.IsAbs(tokensRel) {
		tokensPath = tokensRel
	}
	routingPath := filepath.Join(baseDir, routeRel)
	if filepath.IsAbs(routeRel) {
		routingPath = routeRel
	}

	listenPort := doc.Gateway.ListenPort
	if listenPort == 0 {
		listenPort = defaultListenPort
	}
	listenHost := doc.Gateway.ListenHost
	if listenHost == "" {
		listenHost = defaultListenHost
	}

	ht := doc.Health.TimeoutMs
	if ht == 0 {
		ht = defaultHealthTimeoutMs
	}
	ct := doc.Health.ChatMs
	if ct == 0 {
		ct = defaultChatTimeoutMs
	}

	chain := doc.Routing.FallbackChain
	if chain == nil {
		chain = []string{}
	}
	if len(chain) == 0 && log != nil {
		log.Warn("routing.fallback_chain is empty or missing; virtual model requests will fail until configured")
	}

	logLevel := doc.Gateway.LogLevel
	if logLevel == "" {
		logLevel = defaultLogLevel
	}

	if log != nil {
		log.Debug("resolved gateway config paths", "filePath", filePath, "tokensPath", tokensPath, "routingPolicyPath", routingPath)
	}

	return &Resolved{
		Semver:            semver,
		VirtualModelID:    "Claudia-" + semver,
		ListenPort:        listenPort,
		ListenHost:        listenHost,
		LogLevel:          logLevel,
		LitellmBaseURL:    litellmBase,
		LitellmAPIKeyEnv:  apiKeyEnv,
		HealthLitellmURL:  healthURL,
		HealthTimeoutMs:   ht,
		ChatTimeoutMs:     ct,
		TokensPath:        tokensPath,
		RoutingPolicyPath: routingPath,
		FallbackChain:     chain,
		GatewayYAMLPath:   filePath,
	}, nil
}

// ResolveGatewayConfigPath returns CLAUDIA_GATEWAY_CONFIG or ./config/gateway.yaml relative to cwd.
func ResolveGatewayConfigPath() (string, error) {
	if e := strings.TrimSpace(os.Getenv("CLAUDIA_GATEWAY_CONFIG")); e != "" {
		return filepath.Clean(e), nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, "config", "gateway.yaml"), nil
}

// ListenAddr returns "host:port" for net.Listen.
func (r *Resolved) ListenAddr() string {
	return fmt.Sprintf("%s:%d", r.ListenHost, r.ListenPort)
}
