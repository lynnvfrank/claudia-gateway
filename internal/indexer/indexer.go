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
	"sync"
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

	hooks Hooks
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
	return &Indexer{
		cfg:      cfg,
		client:   client,
		log:      log,
		queue:    NewQueue(cfg.QueueDepth),
		matchers: map[string]*Matcher{},
	}
}

// SetHooks installs test hooks. Must be called before Run.
func (ix *Indexer) SetHooks(h Hooks) { ix.hooks = h }

// Queue exposes the internal queue (read-only intent; tests inspect Len).
func (ix *Indexer) Queue() *Queue { return ix.queue }

// FetchAndLogConfig calls GET /v1/indexer/config and logs version-skew info.
// Failures are logged but do not abort the indexer; the indexer can still
// upload files even if the config endpoint is briefly unavailable.
func (ix *Indexer) FetchAndLogConfig(ctx context.Context) (*IndexerConfig, error) {
	cfg, err := ix.client.FetchConfig(ctx)
	if err != nil {
		ix.log.Warn("fetch indexer config failed", "err", err)
		return nil, err
	}
	ix.log.Info("gateway indexer config",
		"gateway_version", cfg.GatewayVersion,
		"embedding_model", cfg.EmbeddingModel,
		"embedding_dim", cfg.EmbeddingDim,
		"chunk_size", cfg.ChunkSize,
		"chunk_overlap", cfg.ChunkOverlap,
		"max_ingest_bytes", cfg.MaxIngestBytes,
	)
	return cfg, nil
}

// EnqueueInitialScan walks every configured root and pushes a Job for every
// candidate file. Matchers are cached per root for reuse during fs events.
func (ix *Indexer) EnqueueInitialScan(_ context.Context) (int, error) {
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
	hash, _, err := HashFile(j.AbsPath)
	if err != nil {
		return fmt.Errorf("hash %s: %w", j.RelPath, err)
	}
	f, err := os.Open(j.AbsPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", j.RelPath, err)
	}
	defer f.Close()
	res, err := ix.client.Ingest(ctx, IngestRequest{
		Source:      j.RelPath,
		ContentHash: hash,
		Body:        f,
	})
	if err != nil {
		return err
	}
	ix.log.Info("ingested",
		"root", j.Root.ID,
		"rel", j.RelPath,
		"chunks", res.Chunks,
		"collection", res.Collection,
	)
	if ix.hooks.AfterIngest != nil {
		ix.hooks.AfterIngest(j, res)
	}
	return nil
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
			if err == nil && h != nil && h.OK {
				ix.log.Info("storage health recovered; resuming")
				return nil
			}
			if err != nil {
				ix.log.Warn("health probe failed", "err", err)
			} else if h != nil {
				ix.log.Warn("storage health degraded", "status", h.Status, "err", h.Error)
			}
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
