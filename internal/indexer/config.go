// Package indexer implements the v0.2 claudia-index workspace file indexer.
//
// Scope per docs/indexer.plan.md (v0.2):
//   - One or more configured watch roots (directories).
//   - Per-root ignore rules: .claudiaignore + .gitignore + binary detection.
//   - No symlink follow.
//   - Whole-file SHA-256 hashing; one POST /v1/ingest per file.
//   - Bearer token + gateway URL from environment.
//   - Failure handling: bounded exponential backoff; pause and poll
//     /v1/indexer/storage/health when the queue cannot drain.
package indexer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// Defaults aligned with docs/indexer.plan.md § Failure handling.
	defaultRetryAttempts        = 5
	defaultRetryBaseDelay       = 500 * time.Millisecond
	defaultRetryMaxDelay        = 30 * time.Second
	defaultRecoveryPollInterval = 30 * time.Second

	defaultDebounce       = 750 * time.Millisecond
	defaultWorkers        = 4
	defaultQueueDepth     = 1024
	defaultMaxFileBytes   = int64(8 * 1024 * 1024)
	defaultRequestTimeout = 60 * time.Second
)

// EnvGatewayURL and EnvGatewayToken are the v0.2 environment variables for
// gateway connectivity. They map directly to the indexer plan and to the
// Bearer-token model used by the gateway's other APIs.
const (
	EnvGatewayURL   = "CLAUDIA_GATEWAY_URL"
	EnvGatewayToken = "CLAUDIA_GATEWAY_TOKEN"
)

// FileConfig is the on-disk YAML schema (v0.2 minimal).
type FileConfig struct {
	GatewayURL  string   `yaml:"gateway_url"`
	Roots       []string `yaml:"roots"`
	IgnoreExtra []string `yaml:"ignore_extra"`

	RetryMaxAttempts     int     `yaml:"retry_max_attempts"`
	RetryBaseDelayMS     int     `yaml:"retry_base_delay_ms"`
	RetryMaxDelayMS      int     `yaml:"retry_max_delay_ms"`
	RecoveryPollMS       int     `yaml:"recovery_poll_interval_ms"`
	DebounceMS           int     `yaml:"debounce_ms"`
	Workers              int     `yaml:"workers"`
	QueueDepth           int     `yaml:"queue_depth"`
	MaxFileBytes         int64   `yaml:"max_file_bytes"`
	RequestTimeoutMS     int     `yaml:"request_timeout_ms"`
	FollowSymlinks       bool    `yaml:"follow_symlinks"` // v0.2 forces false at Resolve time.
	BinaryNullByteSample int     `yaml:"binary_null_byte_sample_bytes"`
	BinaryNullByteRatio  float64 `yaml:"binary_null_byte_ratio"`
}

// Resolved is the runtime indexer configuration after merging YAML, env vars,
// and CLI overrides. All durations are normalized.
type Resolved struct {
	GatewayURL  string
	Token       string
	Roots       []Root
	IgnoreExtra []string

	RetryMaxAttempts     int
	RetryBaseDelay       time.Duration
	RetryMaxDelay        time.Duration
	RecoveryPollInterval time.Duration
	Debounce             time.Duration
	Workers              int
	QueueDepth           int
	MaxFileBytes         int64
	RequestTimeout       time.Duration

	BinaryNullByteSample int
	BinaryNullByteRatio  float64
}

// Root is a watched directory and its stable, slug-form identifier used in
// logs and (later) for layered project/flavor scoping.
type Root struct {
	// ID is a slug derived from the root's basename; it never appears in
	// payloads sent to the gateway and exists only for local logging.
	ID string
	// AbsPath is the cleaned absolute path on this host. It must never be
	// transmitted to the gateway.
	AbsPath string
}

// LoadFile reads a YAML config file. Returns a zero-value FileConfig if path
// is empty so callers can compose with environment-only setups.
func LoadFile(path string) (FileConfig, error) {
	var fc FileConfig
	if path == "" {
		return fc, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fc, fmt.Errorf("read indexer config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(b, &fc); err != nil {
		return fc, fmt.Errorf("parse indexer config %q: %w", path, err)
	}
	return fc, nil
}

// Overrides captures CLI-flag overrides applied last in the precedence chain.
type Overrides struct {
	GatewayURL string
	Roots      []string
}

// Resolve produces a Resolved config from a parsed file, environment lookup,
// and CLI overrides. Roots and gateway URL fall through layers in this order:
// file < env < overrides. The gateway token is taken from the environment per
// indexer plan v0.2 (no token-in-YAML).
func Resolve(fc FileConfig, env func(string) string, ov Overrides) (Resolved, error) {
	if env == nil {
		env = os.Getenv
	}
	r := Resolved{
		GatewayURL:           strings.TrimSpace(fc.GatewayURL),
		IgnoreExtra:          append([]string(nil), fc.IgnoreExtra...),
		RetryMaxAttempts:     fc.RetryMaxAttempts,
		RetryBaseDelay:       msOr(fc.RetryBaseDelayMS, defaultRetryBaseDelay),
		RetryMaxDelay:        msOr(fc.RetryMaxDelayMS, defaultRetryMaxDelay),
		RecoveryPollInterval: msOr(fc.RecoveryPollMS, defaultRecoveryPollInterval),
		Debounce:             msOr(fc.DebounceMS, defaultDebounce),
		Workers:              fc.Workers,
		QueueDepth:           fc.QueueDepth,
		MaxFileBytes:         fc.MaxFileBytes,
		RequestTimeout:       msOr(fc.RequestTimeoutMS, defaultRequestTimeout),
		BinaryNullByteSample: fc.BinaryNullByteSample,
		BinaryNullByteRatio:  fc.BinaryNullByteRatio,
	}
	if r.RetryMaxAttempts <= 0 {
		r.RetryMaxAttempts = defaultRetryAttempts
	}
	if r.Workers <= 0 {
		r.Workers = defaultWorkers
	}
	if r.QueueDepth <= 0 {
		r.QueueDepth = defaultQueueDepth
	}
	if r.MaxFileBytes <= 0 {
		r.MaxFileBytes = defaultMaxFileBytes
	}
	if r.BinaryNullByteSample <= 0 {
		r.BinaryNullByteSample = 8000
	}
	if r.BinaryNullByteRatio <= 0 || r.BinaryNullByteRatio > 1 {
		r.BinaryNullByteRatio = 0.001 // any NUL byte in the sample marks binary
	}

	if v := strings.TrimSpace(env(EnvGatewayURL)); v != "" {
		r.GatewayURL = v
	}
	if ov.GatewayURL != "" {
		r.GatewayURL = ov.GatewayURL
	}
	r.Token = strings.TrimSpace(env(EnvGatewayToken))

	rawRoots := append([]string{}, fc.Roots...)
	if ov.Roots != nil {
		rawRoots = ov.Roots
	}
	for _, p := range rawRoots {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return r, fmt.Errorf("resolve root %q: %w", p, err)
		}
		abs = filepath.Clean(abs)
		st, err := os.Stat(abs)
		if err != nil {
			return r, fmt.Errorf("stat root %q: %w", abs, err)
		}
		if !st.IsDir() {
			return r, fmt.Errorf("root %q is not a directory", abs)
		}
		r.Roots = append(r.Roots, Root{ID: rootSlug(abs), AbsPath: abs})
	}

	if r.GatewayURL == "" {
		return r, errors.New("gateway URL is required (config gateway_url, --gateway-url, or " + EnvGatewayURL + ")")
	}
	if r.Token == "" {
		return r, errors.New("gateway bearer token is required (set " + EnvGatewayToken + ")")
	}
	if len(r.Roots) == 0 {
		return r, errors.New("at least one watch root is required (config roots or --root)")
	}
	return r, nil
}

func msOr(ms int, def time.Duration) time.Duration {
	if ms <= 0 {
		return def
	}
	return time.Duration(ms) * time.Millisecond
}

// rootSlug returns a short, filesystem-safe identifier derived from the
// basename of an absolute root path. It is intended for human logs only and
// must never be sent to the gateway.
func rootSlug(abs string) string {
	base := filepath.Base(abs)
	base = strings.ToLower(base)
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "root"
	}
	return out
}
