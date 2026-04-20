package indexer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Indexer ties together discovery, watching, the worker pool, and the
// gateway client into a single supervised process. Build one with New, then
// call Run.
type Indexer struct {
	cfg    Resolved
	client *GatewayClient
	log    *slog.Logger

	queue    *Queue
	matchers map[string]*Matcher

	hooks     Hooks
	syncState *SyncState
	lastGW    atomic.Pointer[IndexerConfig]
	// remoteInv is populated from GET /v1/indexer/corpus/inventory during the
	// initial scan (nil when unavailable). Keys are root-relative source paths.
	remoteInv map[string]CorpusInventoryRow
}

// Hooks is an optional set of callbacks tests can install to observe and
// influence the Indexer without wiring real fsnotify or gateway calls.
type Hooks struct {
	// AfterIngest fires once a Job successfully ingests, with the gateway's
	// response.
	AfterIngest func(Job, *IngestResponse)
	// OnSkip fires once per file the walker rejects (binary, ignored,
	// oversize, unreadable).
	OnSkip func(rel, reason string)
	// Now overrides time.Now (sleep timing still uses real clock).
	Now func() time.Time
}

// New constructs an Indexer. The provided log may be nil; a discard logger
// is installed in that case.
func New(cfg Resolved, client *GatewayClient, log *slog.Logger) *Indexer {
	if log == nil {
		log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	st, err := OpenSyncState(cfg.SyncStatePath)
	if err != nil {
		log.Warn("could not open sync state; continuing without skip cache",
			"path", cfg.SyncStatePath, "err", err)
		st = nil
	}
	return &Indexer{
		cfg:       cfg,
		client:    client,
		log:       log,
		queue:     NewQueue(cfg.QueueDepth),
		matchers:  map[string]*Matcher{},
		syncState: st,
	}
}

// SetHooks installs test hooks. Must be called before Run.
func (ix *Indexer) SetHooks(h Hooks) { ix.hooks = h }

// Queue exposes the internal queue (read-only intent; tests inspect Len).
func (ix *Indexer) Queue() *Queue { return ix.queue }

// FetchAndLogConfig calls GET /v1/indexer/config and logs version-skew info.
// Transient failures are logged but do not abort the indexer; a 503 caused by
// the gateway having RAG turned off is surfaced as a fatal error so operators
// see the actionable message instead of watching workers retry forever.
func (ix *Indexer) FetchAndLogConfig(ctx context.Context) (*IndexerConfig, error) {
	cfg, err := ix.client.FetchConfig(ctx, ix.cfg.DefaultIndexerHeaders())
	if err != nil {
		var he *HTTPError
		if errors.As(err, &he) && he.Status == 503 && strings.Contains(strings.ToLower(he.Body), "rag is not enabled") {
			ix.log.Error("gateway has RAG disabled; nothing for the indexer to do",
				"hint", "set rag.enabled=true in config/gateway.yaml and restart the claudia gateway",
				"body", he.Body)
			return nil, err
		}
		ix.log.Warn("fetch indexer config failed", "err", err)
		return nil, err
	}
	ix.lastGW.Store(cfg)
	ix.log.Info("gateway indexer config",
		"gateway_version", cfg.GatewayVersion,
		"embedding_model", cfg.EmbeddingModel,
		"embedding_dim", cfg.EmbeddingDim,
		"chunk_size", cfg.ChunkSize,
		"chunk_overlap", cfg.ChunkOverlap,
		"max_ingest_bytes", cfg.MaxIngestBytes,
		"max_whole_file_bytes", cfg.MaxWholeFileBytes,
		"ingest_session_path", cfg.IngestSessionPath,
		"corpus_inventory_path", cfg.CorpusInventoryPath,
	)
	return cfg, nil
}

// EnqueueInitialScan walks every configured root and pushes a Job for every
// candidate file. Matchers are cached per root for reuse during fs events.
// When gateway config was loaded, it first pulls corpus inventory for
// reconciliation hints (best-effort).
func (ix *Indexer) EnqueueInitialScan(ctx context.Context) (int, error) {
	if err := ix.loadRemoteCorpusInventory(ctx); err != nil {
		ix.log.Warn("corpus inventory fetch skipped", "err", err)
	}
	total := 0
	for _, r := range ix.cfg.Roots {
		m, err := NewMatcher(r.AbsPath, ix.cfg.IgnoreExtra)
		if err != nil {
			return total, fmt.Errorf("ignore matcher for %s: %w", r.AbsPath, err)
		}
		ix.matchers[r.ID] = m
		cands, err := Walk(r, WalkOptions{
			Matcher:              m,
			MaxFileBytes:         ix.cfg.MaxFileBytes,
			BinaryNullByteSample: ix.cfg.BinaryNullByteSample,
			BinaryNullByteRatio:  ix.cfg.BinaryNullByteRatio,
			OnSkip: func(rel, reason string) {
				ix.log.Debug("skip", "root", r.ID, "rel", rel, "reason", reason)
				if ix.hooks.OnSkip != nil {
					ix.hooks.OnSkip(rel, reason)
				}
			},
		})
		if err != nil {
			return total, fmt.Errorf("walk %s: %w", r.AbsPath, err)
		}
		for _, c := range cands {
			if !ix.queue.Enqueue(Job{Root: c.Root, RelPath: c.RelPath, AbsPath: c.AbsPath}) {
				ix.log.Warn("queue full; dropping", "root", c.Root.ID, "rel", c.RelPath)
			}
			total++
		}
	}
	ix.log.Info("initial scan complete", "candidates", total)
	return total, nil
}

func (ix *Indexer) loadRemoteCorpusInventory(ctx context.Context) error {
	gw := ix.lastGW.Load()
	if gw == nil {
		return nil
	}
	p := strings.TrimSpace(gw.CorpusInventoryPath)
	if p == "" {
		p = "/v1/indexer/corpus/inventory"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	m, err := ix.client.FetchCorpusInventoryAll(ctx, p, ix.cfg.DefaultIndexerHeaders())
	if err != nil {
		return err
	}
	ix.remoteInv = m
	ix.log.Info("loaded remote corpus inventory", "sources", len(m))
	return nil
}

// RunWorkers spawns cfg.Workers goroutines that drain the queue. It returns
// when ctx is cancelled or the queue is closed. Workers loop on retryable
// errors per the failure-handling contract; on a fatal error they log and
// drop the job.
func (ix *Indexer) RunWorkers(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < ix.cfg.Workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))
			for {
				j, ok := ix.queue.Dequeue(ctx)
				if !ok {
					return
				}
				if err := ix.processJob(ctx, j, rng); err != nil {
					if errors.Is(err, ErrPaused) {
						ix.log.Warn("worker paused; awaiting health recovery", "worker", id, "rel", j.RelPath)
						if perr := ix.waitForRecovery(ctx); perr != nil {
							return
						}
						_ = ix.queue.Enqueue(j)
						continue
					}
					ix.log.Error("ingest failed (dropped)", "worker", id, "rel", j.RelPath, "err", err)
				}
			}
		}(i)
	}
	wg.Wait()
}

// processJob ingests a single file with bounded retries. It returns
// ErrPaused if all retry attempts fail with a retryable error so the caller
// can switch to recovery polling.
func (ix *Indexer) processJob(ctx context.Context, j Job, rng *rand.Rand) error {
	for attempt := 0; attempt < ix.cfg.RetryMaxAttempts; attempt++ {
		err := ix.ingestOne(ctx, j)
		if err == nil {
			return nil
		}
		if IsFatal(err) {
			return err
		}
		if !IsRetryable(err) {
			return err
		}
		d := Backoff(attempt, ix.cfg.RetryBaseDelay, ix.cfg.RetryMaxDelay, rng)
		ix.log.Warn("ingest retry", "rel", j.RelPath, "attempt", attempt+1, "delay", d, "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	return ErrPaused
}

func (ix *Indexer) ingestOne(ctx context.Context, j Job) error {
	st, err := os.Stat(j.AbsPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", j.RelPath, err)
	}
	if ix.cfg.MaxFileBytes > 0 && st.Size() > ix.cfg.MaxFileBytes {
		return fmt.Errorf("file exceeds max_file_bytes: %s", j.RelPath)
	}
	hash, _, err := HashFile(j.AbsPath)
	if err != nil {
		return fmt.Errorf("hash %s: %w", j.RelPath, err)
	}
	if ix.remoteInv != nil {
		if row, ok := ix.remoteInv[j.RelPath]; ok {
			if row.ClientContentHash != "" && row.ClientContentHash == hash {
				ix.log.Debug("skip unchanged (corpus inventory)", "rel", j.RelPath)
				return nil
			}
			if row.ClientContentHash == "" && ix.syncState != nil {
				if ent, ok := ix.syncState.Get(j.Key()); ok && ent.ServerSHA == row.ContentSHA256 && ent.ClientSHA == hash {
					ix.log.Debug("skip unchanged (corpus inventory + sync state)", "rel", j.RelPath)
					return nil
				}
			}
		}
	}
	if ix.syncState != nil {
		if ent, ok := ix.syncState.Get(j.Key()); ok && ent.ClientSHA == hash {
			ix.log.Debug("skip unchanged (sync state)", "rel", j.RelPath)
			return nil
		}
	}

	gw := ix.lastGW.Load()
	maxIngest := int64(1<<62 - 1)
	if gw != nil && gw.MaxIngestBytes > 0 {
		maxIngest = gw.MaxIngestBytes
	}
	if st.Size() > maxIngest {
		return fmt.Errorf("file larger than gateway max_ingest_bytes (%d): %s", maxIngest, j.RelPath)
	}

	proj, flav := ix.cfg.IngestHeaders(j.Root, j.RelPath)
	wholeLimit := ix.effectiveWholeFileLimit(gw)
	useChunked := gw != nil && strings.TrimSpace(gw.IngestSessionPath) != "" &&
		wholeLimit < maxIngest && st.Size() > wholeLimit

	var res *IngestResponse
	if useChunked {
		pol := SessionRetryPolicy{
			MaxAttempts: ix.cfg.RetryMaxAttempts,
			BaseDelay:   ix.cfg.RetryBaseDelay,
			MaxDelay:    ix.cfg.RetryMaxDelay,
		}
		res, err = ix.client.IngestChunked(ctx, j.AbsPath, IngestRequest{
			Source:      j.RelPath,
			ContentHash: hash,
			Project:     proj,
			Flavor:      flav,
		}, gw, pol)
	} else {
		var f *os.File
		f, err = os.Open(j.AbsPath)
		if err != nil {
			return fmt.Errorf("open %s: %w", j.RelPath, err)
		}
		defer f.Close()
		res, err = ix.client.Ingest(ctx, IngestRequest{
			Source:      j.RelPath,
			ContentHash: hash,
			Project:     proj,
			Flavor:      flav,
			Body:        f,
		})
	}
	if err != nil {
		return err
	}
	serverSHA := strings.TrimSpace(res.ContentSHA256)
	if serverSHA == "" {
		serverSHA = strings.TrimSpace(res.ContentHash)
	}
	if ix.syncState != nil && serverSHA != "" {
		if err := ix.syncState.Put(j.Key(), SyncEntry{ClientSHA: hash, ServerSHA: serverSHA}); err != nil {
			ix.log.Warn("sync state write failed", "rel", j.RelPath, "err", err)
		}
	}
	mode := "whole"
	if useChunked {
		mode = "chunked"
	}
	ix.log.Info("ingested",
		"root", j.Root.ID,
		"rel", j.RelPath,
		"mode", mode,
		"chunks", res.Chunks,
		"collection", res.Collection,
		"content_sha256", serverSHA,
	)
	if ix.hooks.AfterIngest != nil {
		ix.hooks.AfterIngest(j, res)
	}
	return nil
}

func (ix *Indexer) effectiveWholeFileLimit(gw *IndexerConfig) int64 {
	var gwWhole int64
	if gw != nil {
		gwWhole = gw.MaxWholeFileBytes
		if gwWhole <= 0 {
			gwWhole = gw.MaxIngestBytes
		}
	}
	if gwWhole <= 0 {
		gwWhole = ix.cfg.MaxFileBytes
	}
	out := gwWhole
	if ix.cfg.MaxWholeFileBytes > 0 {
		out = min(out, ix.cfg.MaxWholeFileBytes)
	}
	if ix.cfg.MaxFileBytes > 0 {
		out = min(out, ix.cfg.MaxFileBytes)
	}
	return out
}

func (ix *Indexer) waitForRecovery(ctx context.Context) error {
	t := time.NewTicker(ix.cfg.RecoveryPollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			h, err := ix.client.CheckHealth(ctx)
			storageOK := err == nil && h != nil && h.OK
			if err != nil {
				ix.log.Warn("storage health probe failed", "err", err)
			}
			if h != nil && h.RAGDisabled {
				// No amount of waiting will fix this; surface a clear
				// operator-facing message and abort recovery.
				ix.log.Error("gateway has RAG disabled; nothing to recover",
					"detail", h.Message, "type", h.ErrorType,
					"hint", "set rag.enabled=true in config/gateway.yaml and restart the claudia gateway")
				return fmt.Errorf("gateway rejects ingest: %s (%s)", h.Message, h.ErrorType)
			}
			if h != nil && !storageOK && !h.RAGDisabled {
				ix.log.Warn("storage health degraded",
					"status", h.Status, "detail", h.Detail, "http_status", h.HTTPStatus)
			}
			if !storageOK {
				continue
			}
			if ix.cfg.RecoveryIncludeRootHealth {
				rh, rerr := ix.client.CheckGatewayRootHealth(ctx)
				if rerr != nil {
					ix.log.Warn("gateway /health probe failed", "err", rerr)
					continue
				}
				if rh == nil || !rh.OK {
					if rh != nil {
						ix.log.Warn("gateway /health not ready", "status", rh.Status, "degraded", rh.Degraded)
					}
					continue
				}
			}
			ix.log.Info("health recovered; resuming")
			return nil
		}
	}
}

// RunWatchers wires fsnotify watchers onto every configured root and
// translates create/write events into queued jobs (debounced per path).
// Returns when ctx is cancelled.
func (ix *Indexer) RunWatchers(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer w.Close()

	for _, r := range ix.cfg.Roots {
		if err := addRecursive(w, r.AbsPath); err != nil {
			return fmt.Errorf("watch %s: %w", r.AbsPath, err)
		}
	}

	debouncer := newDebouncer(ix.cfg.Debounce, func(absPath string) {
		root, rel, ok := ix.matchAbs(absPath)
		if !ok {
			return
		}
		m := ix.matchers[root.ID]
		if m != nil && m.Match(rel) {
			return
		}
		st, err := os.Stat(absPath)
		if err != nil || !st.Mode().IsRegular() {
			return
		}
		if ix.cfg.MaxFileBytes > 0 && st.Size() > ix.cfg.MaxFileBytes {
			return
		}
		bin, err := IsBinaryFile(absPath, ix.cfg.BinaryNullByteSample, ix.cfg.BinaryNullByteRatio)
		if err != nil || bin {
			return
		}
		_ = ix.queue.Enqueue(Job{Root: root, RelPath: rel, AbsPath: absPath})
	})
	defer debouncer.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				debouncer.Trigger(ev.Name)
			}
			if ev.Op&fsnotify.Create != 0 {
				if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
					_ = addRecursive(w, ev.Name)
				}
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			ix.log.Warn("fsnotify error", "err", err)
		}
	}
}

func addRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return w.Add(p)
		}
		return nil
	})
}

func (ix *Indexer) matchAbs(abs string) (Root, string, bool) {
	for _, r := range ix.cfg.Roots {
		if rel, ok := relPath(r.AbsPath, abs); ok {
			return r, rel, true
		}
	}
	return Root{}, "", false
}
