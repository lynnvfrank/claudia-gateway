package supervisor

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestStartQdrant_KillOnContext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep not used on windows CI")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cfg := QdrantConfig{
		Bin:        "sleep",
		RawExec:    true,
		Args:       []string{"60"},
		StorageDir: t.TempDir(),
		BindHost:   "127.0.0.1",
		HTTPPort:   6333,
		GRPCPort:   6334,
	}
	cmd, err := StartQdrant(ctx, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	waitDone := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(3 * time.Second):
		t.Fatal("qdrant (sleep) did not exit after context cancel")
	}
}

func TestStartQdrant_emptyBin(t *testing.T) {
	_, err := StartQdrant(context.Background(), QdrantConfig{Bin: "", StorageDir: t.TempDir()}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
