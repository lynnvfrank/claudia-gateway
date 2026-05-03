// Package requestid attaches a stable request identifier to each HTTP request context.
package requestid

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type ctxKey struct{}

// FromContext returns the request id or empty string.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(ctxKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// WithContext returns ctx carrying id (typically non-empty).
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// HeaderName is the canonical HTTP header for the request correlation id.
const HeaderName = "X-Request-ID"

// Middleware assigns X-Request-ID when present and valid, otherwise generates a UUID.
// It always echoes the resolved id on the response so clients can tie logs to responses.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(HeaderName))
		if !Valid(id) {
			id = uuid.NewString()
		}
		w.Header().Set(HeaderName, id)
		next.ServeHTTP(w, r.WithContext(WithContext(r.Context(), id)))
	})
}

// Valid reports whether id is safe to log and forward (alphanumeric, hyphen, underscore, dot).
func Valid(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' {
			continue
		}
		return false
	}
	return true
}
