// Package rag wires together the chunker, embedding client, and vector store
// for gateway v0.2 ingest + retrieval. The Service is created once per process
// (after RAG is verified enabled in gateway.yaml) and shared across handlers.
package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lynn/claudia-gateway/internal/rag/chunk"
	"github.com/lynn/claudia-gateway/internal/rag/embed"
	"github.com/lynn/claudia-gateway/internal/vectorstore"
)

// Service orchestrates ingest + retrieval against a single vector store +
// embedding client.
type Service struct {
	store        vectorstore.Store
	embedder     Embedder
	chunkSize    int
	chunkOverlap int
	topK         int
	scoreFloor   float32
	embedDim     int
	log          *slog.Logger
}

// Embedder is the embedding client contract (satisfied by *embed.Client).
type Embedder interface {
	EmbedBatch(ctx context.Context, inputs []string) ([][]float32, error)
	EmbedOne(ctx context.Context, s string) ([]float32, error)
	Model() string
}

// Options configures Service behavior.
type Options struct {
	Store          vectorstore.Store
	Embedder       Embedder
	ChunkSize      int
	ChunkOverlap   int
	TopK           int
	ScoreThreshold float32
	EmbeddingDim   int
	Log            *slog.Logger
}

// New constructs a Service. All store/embedder fields are required.
func New(o Options) (*Service, error) {
	if o.Store == nil {
		return nil, errors.New("rag: nil store")
	}
	if o.Embedder == nil {
		return nil, errors.New("rag: nil embedder")
	}
	if o.ChunkSize <= 0 {
		o.ChunkSize = 512
	}
	if o.ChunkOverlap < 0 || o.ChunkOverlap >= o.ChunkSize {
		o.ChunkOverlap = 128
	}
	if o.TopK <= 0 {
		o.TopK = 8
	}
	if o.EmbeddingDim <= 0 {
		o.EmbeddingDim = 1536
	}
	return &Service{
		store:        o.Store,
		embedder:     o.Embedder,
		chunkSize:    o.ChunkSize,
		chunkOverlap: o.ChunkOverlap,
		topK:         o.TopK,
		scoreFloor:   o.ScoreThreshold,
		embedDim:     o.EmbeddingDim,
		log:          o.Log,
	}, nil
}

// IngestRequest is one document to ingest.
type IngestRequest struct {
	Coords      vectorstore.Coords
	Source      string // relative path or document key
	Text        string
	ContentHash string // optional, client-supplied
}

// IngestResult summarizes what was written.
type IngestResult struct {
	Collection  string
	Source      string
	Chunks      int
	ContentHash string // either client-supplied or server-computed
}

// Ingest chunks → embeds → upserts the document. It ensures the collection
// exists. When ContentHash is empty, a SHA-256 of UTF-8 bytes is computed.
func (s *Service) Ingest(ctx context.Context, req IngestRequest) (IngestResult, error) {
	res := IngestResult{Source: strings.TrimSpace(req.Source)}
	if res.Source == "" {
		return res, errors.New("ingest: empty source")
	}
	if strings.TrimSpace(req.Text) == "" {
		return res, errors.New("ingest: empty text")
	}
	if req.Coords.TenantID == "" {
		return res, errors.New("ingest: empty tenant_id")
	}

	collection := vectorstore.CollectionName(req.Coords)
	res.Collection = collection
	if err := s.store.EnsureCollection(ctx, collection, s.embedDim); err != nil {
		return res, fmt.Errorf("ensure collection %s: %w", collection, err)
	}

	chunks := chunk.Split(req.Text, s.chunkSize, s.chunkOverlap)
	if len(chunks) == 0 {
		return res, errors.New("ingest: no chunks produced")
	}

	inputs := make([]string, 0, len(chunks))
	for _, c := range chunks {
		inputs = append(inputs, c.Text)
	}
	vectors, err := s.embedder.EmbedBatch(ctx, inputs)
	if err != nil {
		return res, fmt.Errorf("embed: %w", err)
	}
	if len(vectors) != len(chunks) {
		return res, fmt.Errorf("embed returned %d vectors for %d chunks", len(vectors), len(chunks))
	}
	for i, v := range vectors {
		if len(v) != s.embedDim {
			return res, fmt.Errorf("embed dim mismatch at chunk %d: got %d, expect %d", i, len(v), s.embedDim)
		}
	}

	// Re-ingest is upsert: delete old points for this source first, then
	// upsert. Errors from delete on a fresh collection are tolerated.
	if err := s.store.DeleteBySource(ctx, collection, res.Source); err != nil && s.log != nil {
		s.log.Debug("delete-by-source pre-ingest failed (likely empty collection)", "source", res.Source, "err", err)
	}

	now := time.Now().Unix()
	pts := make([]vectorstore.Point, 0, len(chunks))
	for i, c := range chunks {
		pts = append(pts, vectorstore.Point{
			ID:     vectorstore.PointID(req.Coords, res.Source, i),
			Vector: vectors[i],
			Payload: vectorstore.Payload{
				TenantID:  req.Coords.TenantID,
				ProjectID: req.Coords.ProjectID,
				FlavorID:  req.Coords.FlavorID,
				Text:      c.Text,
				Source:    res.Source,
				CreatedAt: now,
			},
		})
	}
	if err := s.store.Upsert(ctx, collection, pts); err != nil {
		return res, fmt.Errorf("upsert: %w", err)
	}
	res.Chunks = len(chunks)
	res.ContentHash = strings.TrimSpace(req.ContentHash)
	if res.ContentHash == "" {
		sum := sha256.Sum256([]byte(req.Text))
		res.ContentHash = "sha256:" + hex.EncodeToString(sum[:])
	}
	if s.log != nil {
		s.log.Info("rag ingest", "tenant", req.Coords.TenantID, "project", req.Coords.ProjectID,
			"flavor", req.Coords.FlavorID, "source", res.Source, "chunks", res.Chunks, "collection", collection)
	}
	return res, nil
}

// RetrieveRequest fetches top-k chunks for a query string.
type RetrieveRequest struct {
	Coords vectorstore.Coords
	Query  string
	TopK   int // <= 0 uses Service default
}

// Retrieve embeds the query then runs a top-k search filtered by coords.
func (s *Service) Retrieve(ctx context.Context, req RetrieveRequest) ([]vectorstore.Hit, error) {
	if strings.TrimSpace(req.Query) == "" {
		return nil, nil
	}
	if req.Coords.TenantID == "" {
		return nil, errors.New("retrieve: empty tenant_id")
	}
	k := req.TopK
	if k <= 0 {
		k = s.topK
	}
	collection := vectorstore.CollectionName(req.Coords)
	vec, err := s.embedder.EmbedOne(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	hits, err := s.store.Search(ctx, collection, vec, k, s.scoreFloor, &req.Coords)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	return hits, nil
}

// EmbedDim is the configured embedding dimension (used by /v1/indexer/config).
func (s *Service) EmbedDim() int { return s.embedDim }

// ChunkSize / ChunkOverlap accessors for /v1/indexer/config.
func (s *Service) ChunkSize() int    { return s.chunkSize }
func (s *Service) ChunkOverlap() int { return s.chunkOverlap }
func (s *Service) TopK() int         { return s.topK }

// StoreHealth is exposed for /v1/indexer/storage/health.
func (s *Service) StoreHealth(ctx context.Context) error { return s.store.Health(ctx) }

// StoreStats is exposed for /v1/indexer/storage/stats.
func (s *Service) StoreStats(ctx context.Context, c vectorstore.Coords) (vectorstore.Stats, error) {
	collection := vectorstore.CollectionName(c)
	return s.store.Stats(ctx, collection)
}

// EmbeddingModel returns the configured embedding model id.
func (s *Service) EmbeddingModel() string { return s.embedder.Model() }

// Compile-time guard so embed.Client is a valid Embedder.
var _ Embedder = (*embed.Client)(nil)
