package server

import (
	"github.com/lynn/claudia-gateway/internal/tokens"
)

// BootstrapMode reports whether the gateway should serve the limited bootstrap surface
// (no valid rows in tokens.yaml).
func BootstrapMode(rt *Runtime) bool {
	if rt == nil {
		return true
	}
	_, tokStore, _ := rt.Snapshot()
	if tokStore == nil {
		return true
	}
	return tokens.IsBootstrapMode(tokStore.Path())
}
