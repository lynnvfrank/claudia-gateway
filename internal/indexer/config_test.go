package indexer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_RequiresURLAndToken(t *testing.T) {
	dir := t.TempDir()
	env := func(string) string { return "" }
	_, err := Resolve(FileConfig{Roots: []string{dir}}, env, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "gateway URL") {
		t.Fatalf("expected gateway URL error, got %v", err)
	}
	_, err = Resolve(FileConfig{Roots: []string{dir}, GatewayURL: "http://x"}, env, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("expected token error, got %v", err)
	}
}

func TestResolve_RootMustBeDirectory(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := func(k string) string {
		if k == EnvGatewayToken {
			return "tok"
		}
		return ""
	}
	_, err := Resolve(FileConfig{GatewayURL: "http://x", Roots: []string{file}}, env, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected not-a-directory error, got %v", err)
	}
}

func TestResolve_PrecedenceFileEnvOverride(t *testing.T) {
	dir := t.TempDir()
	envURL := "http://from-env"
	env := func(k string) string {
		switch k {
		case EnvGatewayURL:
			return envURL
		case EnvGatewayToken:
			return "tok"
		}
		return ""
	}
	r, err := Resolve(FileConfig{GatewayURL: "http://from-file", Roots: []string{dir}}, env, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if r.GatewayURL != envURL {
		t.Fatalf("env should win over file: %s", r.GatewayURL)
	}
	r2, err := Resolve(FileConfig{GatewayURL: "http://from-file", Roots: []string{dir}}, env, Overrides{GatewayURL: "http://from-flag"})
	if err != nil {
		t.Fatal(err)
	}
	if r2.GatewayURL != "http://from-flag" {
		t.Fatalf("flag should win over env: %s", r2.GatewayURL)
	}
}

func TestResolve_AppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	env := func(k string) string {
		if k == EnvGatewayToken {
			return "tok"
		}
		return ""
	}
	r, err := Resolve(FileConfig{GatewayURL: "http://x", Roots: []string{dir}}, env, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if r.RetryMaxAttempts != defaultRetryAttempts || r.Workers != defaultWorkers || r.QueueDepth != defaultQueueDepth || r.MaxFileBytes != defaultMaxFileBytes {
		t.Fatalf("defaults not applied: %+v", r)
	}
	if r.Roots[0].ID == "" {
		t.Fatalf("root slug empty")
	}
}
