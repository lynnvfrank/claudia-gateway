package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lynn/claudia-gateway/internal/rag"
	"github.com/lynn/claudia-gateway/internal/vectorstore"
)

// inMemoryStore is a minimal vectorstore.Store for handler integration tests.
type inMemoryStore struct {
	mu          sync.Mutex
	collections map[string]int
	points      map[string][]vectorstore.Point
	healthErr   error
}

func newMemStore() *inMemoryStore {
	return &inMemoryStore{collections: map[string]int{}, points: map[string][]vectorstore.Point{}}
}

func (s *inMemoryStore) EnsureCollection(_ context.Context, name string, dim int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.collections[name]; !ok {
		s.collections[name] = dim
	}
	return nil
}
func (s *inMemoryStore) Upsert(_ context.Context, c string, pts []vectorstore.Point) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.points[c] = append(s.points[c], pts...)
	return nil
}
func (s *inMemoryStore) Search(_ context.Context, c string, _ []float32, k int, _ float32, _ *vectorstore.Coords) ([]vectorstore.Hit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []vectorstore.Hit{}
	for i, p := range s.points[c] {
		if i >= k {
			break
		}
		out = append(out, vectorstore.Hit{ID: p.ID, Score: 0.95, Payload: p.Payload})
	}
	return out, nil
}
func (s *inMemoryStore) Health(context.Context) error { return s.healthErr }
func (s *inMemoryStore) Stats(_ context.Context, c string) (vectorstore.Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return vectorstore.Stats{Collection: c, Points: int64(len(s.points[c])), VectorDim: s.collections[c]}, nil
}
func (s *inMemoryStore) DeleteBySource(_ context.Context, c, src string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	keep := s.points[c][:0]
	for _, p := range s.points[c] {
		if p.Payload.Source != src {
			keep = append(keep, p)
		}
	}
	s.points[c] = keep
	return nil
}

// stubEmbedder yields deterministic dim-sized vectors.
type stubEmbedder struct{ dim int }

func (e stubEmbedder) EmbedBatch(_ context.Context, in []string) ([][]float32, error) {
	out := make([][]float32, len(in))
	for i := range in {
		v := make([]float32, e.dim)
		v[0] = float32(i + 1)
		out[i] = v
	}
	return out, nil
}
func (e stubEmbedder) EmbedOne(ctx context.Context, s string) ([]float32, error) {
	v, err := e.EmbedBatch(ctx, []string{s})
	if err != nil {
		return nil, err
	}
	return v[0], nil
}
func (e stubEmbedder) Model() string { return "test-embed" }

// setupRAGServer wires NewRuntime + a fake RAG service so handler tests run
// without external services. Returns the Runtime, the in-memory store, and a
// running httptest server.
func setupRAGServer(t *testing.T) (*Runtime, *inMemoryStore, *httptest.Server) {
	t.Helper()
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(upstream.Close)

	dir := t.TempDir()
	gwPath := filepath.Join(dir, "gateway.yaml")
	writeGatewayWithRAG(t, gwPath, upstream.URL, []string{"m"}, "http://127.0.0.1:1")
	tokPath := filepath.Join(dir, "tokens.yaml")
	writeTokens(t, tokPath, "ingest-tok", "tenantA")
	routePath := filepath.Join(dir, "routing-policy.yaml")
	if err := os.WriteFile(routePath, []byte("rules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := NewRuntime(gwPath, testLog())
	if err != nil {
		t.Fatal(err)
	}
	store := newMemStore()
	svc, err := rag.New(rag.Options{
		Store:        store,
		Embedder:     stubEmbedder{dim: 8},
		ChunkSize:    128,
		ChunkOverlap: 32,
		TopK:         4,
		EmbeddingDim: 8,
		Log:          testLog(),
	})
	if err != nil {
		t.Fatal(err)
	}
	rt.SetRAGForTest(svc)

	srv := httptest.NewServer(NewMux(rt, testLog(), nil, nil))
	t.Cleanup(srv.Close)
	return rt, store, srv
}

func writeGatewayWithRAG(t *testing.T, path, upstream string, chain []string, qdrantURL string) {
	t.Helper()
	chainYAML := ""
	for _, m := range chain {
		chainYAML += "    - \"" + m + "\"\n"
	}
	raw := "gateway:\n  semver: \"0.2.0\"\n  listen_port: 0\n  listen_host: \"127.0.0.1\"\n" +
		"upstream:\n  base_url: \"" + upstream + "\"\n  api_key_env: \"CLAUDIA_UPSTREAM_API_KEY\"\n" +
		"health:\n  timeout_ms: 2000\n  chat_timeout_ms: 60000\n" +
		"paths:\n  tokens: \"./tokens.yaml\"\n  routing_policy: \"./routing-policy.yaml\"\n" +
		"routing:\n  fallback_chain:\n" + chainYAML +
		"rag:\n  enabled: true\n  qdrant:\n    url: \"" + qdrantURL + "\"\n" +
		"  embedding:\n    model: \"test-embed\"\n    dim: 8\n" +
		"  chunking:\n    size: 128\n    overlap: 32\n" +
		"  ingest:\n    max_bytes: 10485760\n" +
		"  defaults:\n    project_id: \"default\"\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIngest_JSON(t *testing.T) {
	_, store, srv := setupRAGServer(t)
	body := `{"source":"docs/readme.md","text":"` + strings.Repeat("alpha ", 50) + `"}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/ingest", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer ingest-tok")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerProject, "myproj")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var doc map[string]any
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc["tenant_id"] != "tenantA" || doc["project_id"] != "myproj" {
		t.Fatalf("doc: %+v", doc)
	}
	if doc["chunks"].(float64) < 1 {
		t.Fatalf("doc: %+v", doc)
	}
	hash, _ := doc["content_hash"].(string)
	if !strings.HasPrefix(hash, "sha256:") {
		t.Fatalf("content_hash: %q", hash)
	}
	coll, _ := doc["collection"].(string)
	if coll == "" || len(store.points[coll]) == 0 {
		t.Fatalf("no points stored in collection %q", coll)
	}
}

func TestIngest_Multipart(t *testing.T) {
	_, store, srv := setupRAGServer(t)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	w, _ := mw.CreateFormFile("file", "main.go")
	_, _ = w.Write([]byte(strings.Repeat("hello ", 100)))
	_ = mw.WriteField("source", "src/main.go")
	_ = mw.WriteField("content_hash", "sha256:client-supplied")
	_ = mw.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/ingest", &buf)
	req.Header.Set("Authorization", "Bearer ingest-tok")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(headerFlavor, "branch-foo")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var doc map[string]any
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	if doc["flavor_id"] != "branch-foo" {
		t.Fatalf("flavor: %+v", doc)
	}
	if doc["source"] != "src/main.go" {
		t.Fatalf("source should be the explicit form field 'src/main.go', got: %+v", doc)
	}
	if doc["content_hash"] != "sha256:client-supplied" {
		t.Fatalf("content_hash should be client-supplied: %+v", doc)
	}
	coll, _ := doc["collection"].(string)
	if len(store.points[coll]) == 0 {
		t.Fatal("expected points stored")
	}
}

func TestIngest_Unauthorized(t *testing.T) {
	_, _, srv := setupRAGServer(t)
	body := `{"source":"a","text":"hi"}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/ingest", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestIngest_RAGDisabled_503(t *testing.T) {
	t.Setenv("CLAUDIA_UPSTREAM_API_KEY", "ukey")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(upstream.Close)
	dir := t.TempDir()
	gwPath := filepath.Join(dir, "gateway.yaml")
	writeGateway(t, gwPath, upstream.URL, []string{"m"})
	tokPath := filepath.Join(dir, "tokens.yaml")
	writeTokens(t, tokPath, "tok", "ten")
	routePath := filepath.Join(dir, "routing-policy.yaml")
	_ = os.WriteFile(routePath, []byte("rules: []\n"), 0o644)
	rt, err := NewRuntime(gwPath, testLog())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(NewMux(rt, testLog(), nil, nil))
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/ingest", strings.NewReader(`{"source":"a","text":"x"}`))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestIngest_BadBody(t *testing.T) {
	_, _, srv := setupRAGServer(t)
	cases := []struct {
		name string
		ct   string
		body string
		want int
	}{
		{"empty json", "application/json", `{}`, http.StatusBadRequest},
		{"missing source", "application/json", `{"text":"x"}`, http.StatusBadRequest},
		{"bad ct", "text/plain", "hello", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/ingest", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer ingest-tok")
			req.Header.Set("Content-Type", tc.ct)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != tc.want {
				t.Fatalf("got %d want %d", res.StatusCode, tc.want)
			}
		})
	}
}
