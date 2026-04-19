package indexer

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_FetchConfigAndHealth(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/indexer/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"gateway_version":"v0.2","embedding_model":"m","embedding_dim":8,"chunk_size":512,"chunk_overlap":128,"ingest_path":"/v1/ingest","max_ingest_bytes":1024}`))
	})
	mux.HandleFunc("/v1/indexer/storage/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"status":"ready"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewGatewayClient(srv.URL, "tok", 5*time.Second)
	ctx := context.Background()
	cfg, err := c.FetchConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EmbeddingDim != 8 || cfg.ChunkSize != 512 {
		t.Fatalf("cfg=%+v", cfg)
	}
	h, err := c.CheckHealth(ctx)
	if err != nil || !h.OK {
		t.Fatalf("health=%+v err=%v", h, err)
	}
}

func TestClient_Ingest_Multipart(t *testing.T) {
	var seenSource, seenHash, seenFilename, seenBody string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/ingest", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mt, "multipart/") {
			http.Error(w, "bad ct", http.StatusBadRequest)
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			b, _ := io.ReadAll(p)
			switch p.FormName() {
			case "source":
				seenSource = string(b)
			case "content_hash":
				seenHash = string(b)
			case "file":
				seenFilename = p.FileName()
				seenBody = string(b)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"ingest.result","tenant_id":"t","project_id":"p","flavor_id":"f","source":"src/main.go","content_hash":"sha256:abc","chunks":3,"collection":"c"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := NewGatewayClient(srv.URL, "tok", 5*time.Second)
	res, err := c.Ingest(context.Background(), IngestRequest{
		Source:      "src/main.go",
		ContentHash: "sha256:abc",
		Body:        strings.NewReader("hello"),
		Project:     "p",
		Flavor:      "f",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Chunks != 3 {
		t.Fatalf("chunks=%d", res.Chunks)
	}
	if seenSource != "src/main.go" || seenHash != "sha256:abc" || seenBody != "hello" || seenFilename != "main.go" {
		t.Fatalf("multipart fields: source=%q hash=%q body=%q file=%q", seenSource, seenHash, seenBody, seenFilename)
	}
}

func TestClient_RetryClassification(t *testing.T) {
	cases := []struct {
		status       int
		retry, fatal bool
	}{
		{http.StatusServiceUnavailable, true, false},
		{http.StatusTooManyRequests, true, false},
		{http.StatusInternalServerError, true, false},
		{http.StatusUnauthorized, false, true},
		{http.StatusForbidden, false, true},
		{http.StatusBadRequest, false, true},
	}
	for _, c := range cases {
		err := &HTTPError{Path: "/v1/ingest", Status: c.status}
		if got := IsRetryable(err); got != c.retry {
			t.Fatalf("status %d: retry=%v want %v", c.status, got, c.retry)
		}
		if got := IsFatal(err); got != c.fatal {
			t.Fatalf("status %d: fatal=%v want %v", c.status, got, c.fatal)
		}
	}
}

func TestClient_Ingest_RetryableThenSuccess(t *testing.T) {
	var calls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/ingest", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chunks":1}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := NewGatewayClient(srv.URL, "tok", 5*time.Second)

	var lastErr error
	var resp *IngestResponse
	for attempt := 0; attempt < 5; attempt++ {
		resp, lastErr = c.Ingest(context.Background(), IngestRequest{
			Source: "a.txt", Body: strings.NewReader("x"),
		})
		if lastErr == nil {
			break
		}
		if !IsRetryable(lastErr) {
			t.Fatalf("non-retryable err on attempt %d: %v", attempt, lastErr)
		}
	}
	if lastErr != nil {
		t.Fatalf("expected eventual success, got %v", lastErr)
	}
	if resp == nil || resp.Chunks != 1 || atomic.LoadInt32(&calls) < 3 {
		t.Fatalf("resp=%+v calls=%d", resp, calls)
	}
}

// Sanity: HTTPError formats and surfaces JSON status fields.
func TestHTTPError_String(t *testing.T) {
	e := &HTTPError{Path: "/v1/x", Status: 503, Body: `{"error":"busy"}`}
	if !strings.Contains(e.Error(), "503") || !strings.Contains(e.Error(), "/v1/x") {
		t.Fatalf("err: %s", e.Error())
	}
}

// Ensure the JSON tags survive a round-trip; cheap regression check on the
// IngestResponse shape we depend on.
func TestIngestResponse_JSONShape(t *testing.T) {
	in := IngestResponse{Object: "ingest.result", Chunks: 2}
	b, _ := json.Marshal(in)
	var out IngestResponse
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Chunks != 2 || out.Object != "ingest.result" {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
}
