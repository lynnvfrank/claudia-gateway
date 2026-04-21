package requestid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValid(t *testing.T) {
	if !Valid("abc-123.X_y") {
		t.Fatal("expected valid")
	}
	if Valid("") || Valid("bad id") || Valid(strings.Repeat("a", 200)) {
		t.Fatal("expected invalid")
	}
}

func TestMiddleware_usesHeaderWhenValid(t *testing.T) {
	var got string
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "client-req-1")
	h.ServeHTTP(rec, req)
	if got != "client-req-1" {
		t.Fatalf("got %q", got)
	}
}

func TestMiddleware_generatesUUID(t *testing.T) {
	var got string
	h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = FromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got == "" || len(got) < 8 {
		t.Fatalf("expected generated id, got %q", got)
	}
}

func TestFromContext_empty(t *testing.T) {
	if FromContext(context.Background()) != "" {
		t.Fatal("expected empty")
	}
}
