package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lynn/claudia-gateway/internal/platform/requestid"
)

func TestLoggingMiddleware_emitsRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	h := requestid.Middleware(loggingMiddleware(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(rec, req)
	out := buf.String()
	if !strings.Contains(out, "request_id=") {
		t.Fatalf("missing request_id in log: %q", out)
	}
	if !strings.Contains(out, "service=gateway") {
		t.Fatalf("missing service=gateway: %q", out)
	}
	if !strings.Contains(out, "statusCode=418") {
		t.Fatalf("missing status: %q", out)
	}
}

func TestOptionalConversationIDFromHeader_set(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.Header.Set(headerConversationID, "sess-abc-1")
	if got := optionalConversationIDFromHeader(r); got != "sess-abc-1" {
		t.Fatalf("got %q", got)
	}
}

func TestOptionalConversationIDFromHeader_empty(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if got := optionalConversationIDFromHeader(r); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}
