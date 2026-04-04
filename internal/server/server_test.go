package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"log/slog"

	"github.com/lynn/claudia-gateway/internal/config"
)

func testLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

func TestStatusEndpoint(t *testing.T) {
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(up.Close)

	dir := t.TempDir()
	gwPath := filepath.Join(dir, "gateway.yaml")
	writeGateway(t, gwPath, up.URL, []string{"m"})
	tokPath := filepath.Join(dir, "tokens.yaml")
	writeTokens(t, tokPath, "t", "x")
	routePath := filepath.Join(dir, "routing-policy.yaml")
	if err := os.WriteFile(routePath, []byte("rules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := NewRuntime(gwPath, testLog())
	if err != nil {
		t.Fatal(err)
	}
	ov := &StatusOverlay{
		EffectiveListen: "127.0.0.1:3999",
		Supervisor: &SupervisorInfo{
			BifrostListen:    "127.0.0.1:8080",
			QdrantSupervised: false,
		},
	}
	srv := httptest.NewServer(NewMux(rt, testLog(), ov))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var doc map[string]any
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	sup, _ := doc["supervisor"].(map[string]any)
	if sup["active"] != true {
		t.Fatalf("supervisor: %+v", sup)
	}
	gw, _ := doc["gateway"].(map[string]any)
	if gw["listen"] != "127.0.0.1:3999" {
		t.Fatalf("gateway.listen: %+v", gw)
	}
}

func TestListenAddrOverride(t *testing.T) {
	r := &config.Resolved{ListenHost: "127.0.0.1", ListenPort: 3000}
	if ListenAddrOverride(r, "") != "127.0.0.1:3000" {
		t.Fatal(ListenAddrOverride(r, ""))
	}
	if ListenAddrOverride(r, ":4000") != "127.0.0.1:4000" {
		t.Fatal()
	}
	if ListenAddrOverride(r, "0.0.0.0:9") != "0.0.0.0:9" {
		t.Fatal()
	}
}

func TestChatVirtualModelFallback429(t *testing.T) {
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")

	var seenModels []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case "/v1/chat/completions":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			m, _ := body["model"].(string)
			seenModels = append(seenModels, m)
			if m == "groq/a" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(up.Close)

	dir := t.TempDir()
	gwPath := filepath.Join(dir, "gateway.yaml")
	writeGateway(t, gwPath, up.URL, []string{"groq/a", "groq/b"})
	tokPath := filepath.Join(dir, "tokens.yaml")
	writeTokens(t, tokPath, "secret-gw", "t1")
	routePath := filepath.Join(dir, "routing-policy.yaml")
	writeRouting(t, routePath, "groq/a", 999999) // no rule match for short message → ambiguous or chain

	rt, err := NewRuntime(gwPath, testLog())
	if err != nil {
		t.Fatal(err)
	}
	h := NewMux(rt, testLog(), nil)
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)

	reqBody := `{"model":"Claudia-0.1.0","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req, _ := http.NewRequest(http.MethodPost, front.URL+"/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Authorization", "Bearer secret-gw")
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	if len(seenModels) < 2 {
		t.Fatalf("expected retry, seenModels=%v", seenModels)
	}
	if seenModels[0] != "groq/a" || seenModels[1] != "groq/b" {
		t.Fatalf("order: %v", seenModels)
	}
}

func writeGateway(t *testing.T, path, upstream string, chain []string) {
	t.Helper()
	chainYAML := ""
	for _, m := range chain {
		chainYAML += "    - \"" + m + "\"\n"
	}
	raw := "gateway:\n  semver: \"0.1.0\"\n  listen_port: 0\n  listen_host: \"127.0.0.1\"\n" +
		"upstream:\n  base_url: \"" + upstream + "\"\n  api_key_env: \"CLAUDIA_UPSTREAM_API_KEY\"\n" +
		"health:\n  timeout_ms: 2000\n  chat_timeout_ms: 60000\n" +
		"paths:\n  tokens: \"./tokens.yaml\"\n  routing_policy: \"./routing-policy.yaml\"\n" +
		"routing:\n  fallback_chain:\n" + chainYAML
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTokens(t *testing.T, path, token, tenant string) {
	t.Helper()
	raw := "tokens:\n  - token: \"" + token + "\"\n    tenant_id: \"" + tenant + "\"\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRouting(t *testing.T, path, model string, minChars int) {
	t.Helper()
	raw := "ambiguous_default_model: \"" + model + "\"\nrules:\n  - name: x\n    when:\n      min_message_chars: " +
		strconv.Itoa(minChars) + "\n    models:\n      - \"" + model + "\"\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}
