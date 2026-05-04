package supervisor

import (
	"bytes"
	"context"
	"runtime"
	"testing"
	"time"
)

func TestStartIndexer_RawExecImmediateExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var bin string
	var args []string
	if runtime.GOOS == "windows" {
		bin = "cmd.exe"
		args = []string{"/c", "exit", "0"}
	} else {
		bin = "true"
	}

	var stdout, stderr bytes.Buffer
	cmd, err := StartIndexer(ctx, IndexerConfig{
		RawExec: true,
		Bin:     bin,
		Args:    args,
		Stdout:  &stdout,
		Stderr:  &stderr,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait: %v stderr=%q", err, stderr.String())
	}
}
