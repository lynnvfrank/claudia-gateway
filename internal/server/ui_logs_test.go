package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lynn/claudia-gateway/internal/servicelogs"
)

func bifrostStubForUILogs(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/api/providers/groq":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"groq","keys":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func runtimeForUILogs(t *testing.T, bifrostURL string) *Runtime {
	t.Helper()
	dir := t.TempDir()
	gwPath := filepath.Join(dir, "gateway.yaml")
	writeGateway(t, gwPath, bifrostURL, []string{"m"})
	tokPath := filepath.Join(dir, "tokens.yaml")
	writeTokens(t, tokPath, "gw-ui-secret", "t1")
	routePath := filepath.Join(dir, "routing-policy.yaml")
	if err := os.WriteFile(routePath, []byte("rules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := NewRuntime(gwPath, testLog())
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

func TestUILogsAPI_unauthorizedWithoutSession(t *testing.T) {
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")
	up := bifrostStubForUILogs(t)
	t.Cleanup(up.Close)

	rt := runtimeForUILogs(t, up.URL)
	logStore := servicelogs.New(100)
	ui := NewUIOptions()
	ui.LogStore = logStore
	front := httptest.NewServer(NewMux(rt, testLog(), nil, ui))
	t.Cleanup(front.Close)

	res, err := http.Get(front.URL + "/api/ui/logs?since=0")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("logs poll: status %d", res.StatusCode)
	}

	res2, err := http.Get(front.URL + "/api/ui/logs/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("logs stream: status %d", res2.StatusCode)
	}
}

func TestUILogsPoll_returnsLinesAfterSince(t *testing.T) {
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")
	up := bifrostStubForUILogs(t)
	t.Cleanup(up.Close)

	rt := runtimeForUILogs(t, up.URL)
	logStore := servicelogs.New(100)
	_, _ = io.WriteString(logStore.Writer("unit"), "alpha\nbeta\n")

	ui := NewUIOptions()
	ui.LogStore = logStore
	front := httptest.NewServer(NewMux(rt, testLog(), nil, ui))
	t.Cleanup(front.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	if _, err := client.Post(front.URL+"/api/ui/login", "application/json", strings.NewReader(`{"token":"gw-ui-secret"}`)); err != nil {
		t.Fatal(err)
	}

	res, err := client.Get(front.URL + "/api/ui/logs?since=0")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body logsPollResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Lines) != 2 {
		t.Fatalf("lines: %+v", body.Lines)
	}
	if body.Lines[0].Text != "alpha" || body.Lines[1].Text != "beta" {
		t.Fatalf("content: %+v", body.Lines)
	}
	if body.MaxSeq != 2 {
		t.Fatalf("max_seq %d", body.MaxSeq)
	}

	res2, err := client.Get(front.URL + "/api/ui/logs?since=1")
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	var body2 logsPollResponse
	if err := json.NewDecoder(res2.Body).Decode(&body2); err != nil {
		t.Fatal(err)
	}
	if len(body2.Lines) != 1 || body2.Lines[0].Text != "beta" {
		t.Fatalf("since=1: %+v", body2.Lines)
	}
}

func TestUILogsStream_replaysTailOnConnect(t *testing.T) {
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")
	up := bifrostStubForUILogs(t)
	t.Cleanup(up.Close)

	rt := runtimeForUILogs(t, up.URL)
	logStore := servicelogs.New(100)
	_, _ = io.WriteString(logStore.Writer("unit"), "sse-seed\n")

	ui := NewUIOptions()
	ui.LogStore = logStore
	h := NewMux(rt, testLog(), nil, ui)
	front := httptest.NewServer(h)
	t.Cleanup(front.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	if _, err := client.Post(front.URL+"/api/ui/login", "application/json", strings.NewReader(`{"token":"gw-ui-secret"}`)); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(front.URL)
	if err != nil {
		t.Fatal(err)
	}
	var sessionValue string
	for _, c := range jar.Cookies(u) {
		if c.Name == defaultUICookieName {
			sessionValue = c.Value
			break
		}
	}
	if sessionValue == "" {
		t.Fatal("no session cookie after login")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ui/logs/stream", nil)
	req = req.WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: defaultUICookieName, Value: sessionValue})
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stream status %d body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "sse-seed") {
		t.Fatalf("expected SSE replay, got %q", body[:min(400, len(body))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestUILogsPage_requiresAuth(t *testing.T) {
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")
	up := bifrostStubForUILogs(t)
	t.Cleanup(up.Close)

	rt := runtimeForUILogs(t, up.URL)
	ui := NewUIOptions()
	ui.LogStore = servicelogs.New(10)
	front := httptest.NewServer(NewMux(rt, testLog(), nil, ui))
	t.Cleanup(front.Close)

	noFollow := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := noFollow.Get(front.URL + "/ui/logs")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("status %d", res.StatusCode)
	}
	loc := res.Header.Get("Location")
	if !strings.Contains(loc, "/ui/login") || !strings.Contains(loc, "next=") {
		t.Fatalf("location %q", loc)
	}
}

func TestUIDesktopPage_requiresAuth(t *testing.T) {
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")
	up := bifrostStubForUILogs(t)
	t.Cleanup(up.Close)

	rt := runtimeForUILogs(t, up.URL)
	ui := NewUIOptions()
	ui.LogStore = servicelogs.New(10)
	front := httptest.NewServer(NewMux(rt, testLog(), nil, ui))
	t.Cleanup(front.Close)

	noFollow := &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err := noFollow.Get(front.URL + "/ui/desktop")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusFound {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestUILogsPage_servesLogsHTMLWhenAuthed(t *testing.T) {
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")
	up := bifrostStubForUILogs(t)
	t.Cleanup(up.Close)

	rt := runtimeForUILogs(t, up.URL)
	ui := NewUIOptions()
	ui.LogStore = servicelogs.New(10)
	front := httptest.NewServer(NewMux(rt, testLog(), nil, ui))
	t.Cleanup(front.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	if _, err := client.Post(front.URL+"/api/ui/login", "application/json", strings.NewReader(`{"token":"gw-ui-secret"}`)); err != nil {
		t.Fatal(err)
	}
	res, err := client.Get(front.URL + "/ui/logs")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	if !strings.Contains(page, "Claudia — Logs") || !strings.Contains(page, "EventSource") {
		snippet := page
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		t.Fatalf("unexpected page: %q", snippet)
	}
}

func TestUIDesktopPage_servesShellWhenAuthed(t *testing.T) {
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")
	up := bifrostStubForUILogs(t)
	t.Cleanup(up.Close)

	rt := runtimeForUILogs(t, up.URL)
	ui := NewUIOptions()
	ui.LogStore = servicelogs.New(10)
	front := httptest.NewServer(NewMux(rt, testLog(), nil, ui))
	t.Cleanup(front.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	if _, err := client.Post(front.URL+"/api/ui/login", "application/json", strings.NewReader(`{"token":"gw-ui-secret"}`)); err != nil {
		t.Fatal(err)
	}
	res, err := client.Get(front.URL + "/ui/desktop")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	if !strings.Contains(page, "f-logs") || !strings.Contains(page, "/ui/logs") {
		t.Fatal("expected tabbed shell markup")
	}
	if !strings.Contains(page, "f-stats") || !strings.Contains(page, "/ui/metrics") {
		t.Fatal("expected stats tab / metrics iframe")
	}
}
