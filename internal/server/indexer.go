package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/lynn/claudia-gateway/internal/vectorstore"
)

// handleIndexerConfig returns the effective RAG / indexer settings for the
// authenticated tenant (per docs/version-v0.2.md "Indexer REST").
func handleIndexerConfig(w http.ResponseWriter, r *http.Request, rt *Runtime, _ *slog.Logger) {
	rt.Sync()
	res, tokStore, _ := rt.Snapshot()
	token := bearerToken(r.Header.Get("Authorization"))
	sess := tokStore.Validate(token)
	if token == "" || sess == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized", "invalid_api_key")
		return
	}
	if !res.RAG.Enabled || rt.RAG() == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "RAG is not enabled", "gateway_config")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object":           "indexer.config",
		"gateway_version":  res.Semver,
		"chunk_size":       rt.RAG().ChunkSize(),
		"chunk_overlap":    rt.RAG().ChunkOverlap(),
		"top_k":            rt.RAG().TopK(),
		"score_threshold":  res.RAG.ScoreThreshold,
		"embedding_model":  rt.RAG().EmbeddingModel(),
		"embedding_dim":    rt.RAG().EmbedDim(),
		"ingest_method":    "POST",
		"ingest_path":      "/v1/ingest",
		"max_ingest_bytes": res.RAG.MaxIngestBytes,
		"required_headers": []string{"Authorization"},
		"optional_headers": []string{headerProject, headerFlavor},
		"payload_fields":   []string{"tenant_id", "project_id", "text", "source", "flavor_id", "created_at"},
		"collection_naming": map[string]any{
			"scheme":  "claudia-<tenant>-<project>-<flavor>-<sha1prefix>",
			"scope":   res.RAG.CollectionScope,
			"example": vectorstore.CollectionName(vectorstore.Coords{TenantID: sess.TenantID, ProjectID: defaultOr(res.RAG.DefaultProject, "default"), FlavorID: res.RAG.DefaultFlavor}),
		},
		"defaults": map[string]any{
			"project_id": res.RAG.DefaultProject,
			"flavor_id":  res.RAG.DefaultFlavor,
		},
	})
}

// handleIndexerHealth probes the configured vector store. Always scoped to the
// authenticated tenant in the response, even though Qdrant itself is shared.
func handleIndexerHealth(w http.ResponseWriter, r *http.Request, rt *Runtime, _ *slog.Logger) {
	rt.Sync()
	res, tokStore, _ := rt.Snapshot()
	token := bearerToken(r.Header.Get("Authorization"))
	sess := tokStore.Validate(token)
	if token == "" || sess == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized", "invalid_api_key")
		return
	}
	if !res.RAG.Enabled || rt.RAG() == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "RAG is not enabled", "gateway_config")
		return
	}
	err := rt.RAG().StoreHealth(r.Context())
	resp := map[string]any{
		"object":    "indexer.storage.health",
		"backend":   "qdrant",
		"url":       res.RAG.QdrantURL,
		"tenant_id": sess.TenantID,
	}
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		resp["status"] = "degraded"
		resp["ok"] = false
		resp["detail"] = err.Error()
		_ = json.NewEncoder(w).Encode(resp)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	resp["status"] = "ok"
	resp["ok"] = true
	_ = json.NewEncoder(w).Encode(resp)
}

// handleIndexerStats returns live Qdrant stats for the (tenant, project,
// flavor) collection that the request scope resolves to.
func handleIndexerStats(w http.ResponseWriter, r *http.Request, rt *Runtime, _ *slog.Logger) {
	rt.Sync()
	res, tokStore, _ := rt.Snapshot()
	token := bearerToken(r.Header.Get("Authorization"))
	sess := tokStore.Validate(token)
	if token == "" || sess == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized", "invalid_api_key")
		return
	}
	if !res.RAG.Enabled || rt.RAG() == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "RAG is not enabled", "gateway_config")
		return
	}
	coords := vectorstore.Coords{
		TenantID:  sess.TenantID,
		ProjectID: resolveProject(r.Header.Get(headerProject), res.RAG.DefaultProject),
		FlavorID:  resolveFlavor(r.Header.Get(headerFlavor), res.RAG.DefaultFlavor),
	}
	st, err := rt.RAG().StoreStats(r.Context(), coords)
	if err != nil {
		// Treat 404 / missing collection as zero-points (hasn't ingested yet).
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object":     "indexer.storage.stats",
			"collection": vectorstore.CollectionName(coords),
			"tenant_id":  coords.TenantID,
			"project_id": coords.ProjectID,
			"flavor_id":  coords.FlavorID,
			"points":     0,
			"vector_dim": rt.RAG().EmbedDim(),
			"available":  false,
			"detail":     err.Error(),
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object":     "indexer.storage.stats",
		"collection": st.Collection,
		"tenant_id":  coords.TenantID,
		"project_id": coords.ProjectID,
		"flavor_id":  coords.FlavorID,
		"points":     st.Points,
		"vector_dim": st.VectorDim,
		"available":  true,
	})
}

func defaultOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
