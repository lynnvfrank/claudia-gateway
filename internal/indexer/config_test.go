package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_RequiresURLAndToken(t *testing.T) {
	dir := t.TempDir()
	env := func(string) string { return "" }
	_, err := Resolve(FileConfig{Roots: FlexibleRoots{{Path: dir}}}, env, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "gateway URL") {
		t.Fatalf("expected gateway URL error, got %v", err)
	}
	_, err = Resolve(FileConfig{Roots: FlexibleRoots{{Path: dir}}, GatewayURL: "http://x"}, env, Overrides{})
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
	_, err := Resolve(FileConfig{GatewayURL: "http://x", Roots: FlexibleRoots{{Path: file}}}, env, Overrides{})
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
	r, err := Resolve(FileConfig{GatewayURL: "http://from-file", Roots: FlexibleRoots{{Path: dir}}}, env, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if r.GatewayURL != envURL {
		t.Fatalf("env should win over file: %s", r.GatewayURL)
	}
	r2, err := Resolve(FileConfig{GatewayURL: "http://from-file", Roots: FlexibleRoots{{Path: dir}}}, env, Overrides{GatewayURL: "http://from-flag"})
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
	r, err := Resolve(FileConfig{GatewayURL: "http://x", Roots: FlexibleRoots{{Path: dir}}}, env, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if r.RetryMaxAttempts != defaultRetryAttempts || r.Workers != defaultWorkers || r.QueueDepth != defaultQueueDepth || r.MaxFileBytes != defaultMaxFileBytes {
		t.Fatalf("defaults not applied: %+v", r)
	}
	if r.Roots[0].ID == "" {
		t.Fatalf("root slug empty")
	}
	if !r.RecoveryIncludeRootHealth {
		t.Fatal("expected RecoveryIncludeRootHealth default true")
	}
}

func TestResolve_RecoveryIncludeRootHealthFalse(t *testing.T) {
	dir := t.TempDir()
	env := func(k string) string {
		if k == EnvGatewayToken {
			return "tok"
		}
		return ""
	}
	f := false
	r, err := Resolve(FileConfig{
		GatewayURL:                "http://x",
		Roots:                     FlexibleRoots{{Path: dir}},
		RecoveryIncludeRootHealth: &f,
	}, env, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if r.RecoveryIncludeRootHealth {
		t.Fatal("expected false from YAML")
	}
}

func TestMergeFileConfig_LayerOrder(t *testing.T) {
	a := FileConfig{GatewayURL: "http://a", RetryMaxAttempts: 3, Roots: FlexibleRoots{{Path: "/x"}}}
	b := FileConfig{GatewayURL: "http://b", RetryMaxAttempts: 9}
	out := MergeFileConfig(a, b)
	if out.GatewayURL != "http://b" || out.RetryMaxAttempts != 9 {
		t.Fatalf("got %+v", out)
	}
	if len(out.Roots) != 1 || out.Roots[0].Path != "/x" {
		t.Fatalf("roots should fall through when overlay empty: %+v", out.Roots)
	}
}

func TestLoadLayeredConfig_MergesGlobalThenLocal(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	rootReal := filepath.Join(cwd, "proj")
	if err := os.MkdirAll(rootReal, 0o755); err != nil {
		t.Fatal(err)
	}
	globalPath := filepath.Join(home, ".claudia", "indexer.config.yaml")
	localPath := filepath.Join(cwd, ".claudia", "indexer.config.yaml")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		t.Fatal(err)
	}
	gy := fmt.Sprintf("gateway_url: http://global\nroots:\n  - %q\n", rootReal)
	if err := os.WriteFile(globalPath, []byte(gy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(localPath, []byte("gateway_url: http://local\nretry_max_attempts: 7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fc, err := LoadLayeredConfig(cwd, "")
	if err != nil {
		t.Fatal(err)
	}
	if fc.GatewayURL != "http://local" {
		t.Fatalf("gateway=%q", fc.GatewayURL)
	}
	if fc.RetryMaxAttempts != 7 {
		t.Fatalf("retry=%d", fc.RetryMaxAttempts)
	}
	if len(fc.Roots) != 1 || fc.Roots[0].Path != rootReal {
		t.Fatalf("roots=%+v", fc.Roots)
	}
}

func TestLoadLayeredConfig_ExplicitMissingErrors(t *testing.T) {
	_, err := LoadLayeredConfig(t.TempDir(), filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err=%v", err)
	}
}

func TestResolve_InvalidOverrideGlob(t *testing.T) {
	dir := t.TempDir()
	env := func(k string) string {
		if k == EnvGatewayToken {
			return "tok"
		}
		return ""
	}
	_, err := Resolve(FileConfig{
		GatewayURL: "http://x",
		Roots:      FlexibleRoots{{Path: dir}},
		Overrides:  []OverrideYAML{{Glob: "[[bad"}},
	}, env, Overrides{})
	if err == nil || !strings.Contains(err.Error(), "invalid glob") {
		t.Fatalf("expected invalid glob error, got %v", err)
	}
}

func TestLoadFile_RootsStringsAndObjects(t *testing.T) {
	base := t.TempDir()
	a := filepath.Join(base, "alpha")
	b := filepath.Join(base, "beta")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(base, "indexer.config.yaml")
	yaml := fmt.Sprintf(`gateway_url: http://gw.example
roots:
  - %q
  - path: %q
    project_id: webapp
    flavor_id: webflav
`, a, b)
	if err := os.WriteFile(cfgPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	fc, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(fc.Roots) != 2 {
		t.Fatalf("roots len=%d", len(fc.Roots))
	}
	if fc.Roots[0].Path != a || fc.Roots[0].ProjectID != "" {
		t.Fatalf("root0=%+v", fc.Roots[0])
	}
	if fc.Roots[1].Path != b || fc.Roots[1].ProjectID != "webapp" || fc.Roots[1].FlavorID != "webflav" {
		t.Fatalf("root1=%+v", fc.Roots[1])
	}
	env := func(k string) string {
		if k == EnvGatewayToken {
			return "tok"
		}
		return ""
	}
	r, err := Resolve(fc, env, Overrides{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Roots) != 2 {
		t.Fatal(len(r.Roots))
	}
	if r.Roots[1].Scope.ProjectID != "webapp" || r.Roots[1].Scope.FlavorID != "webflav" {
		t.Fatalf("resolved root1 scope=%+v", r.Roots[1].Scope)
	}
}
