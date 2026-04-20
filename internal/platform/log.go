// Package platform exposes small primitives shared across the gateway that
// don't belong to a specific feature package. The log helpers here define the
// gateway-wide custom slog levels (notably TRACE, which slog does not provide
// out of the box).
package platform

import "log/slog"

// LevelTrace is one step below slog.LevelDebug (-4). Use for very chatty
// per-operation traces (RAG ingest results, embedding payload sizes, etc.)
// that are too noisy for DEBUG. Mirrors common conventions in zap / logrus.
const LevelTrace slog.Level = -8
