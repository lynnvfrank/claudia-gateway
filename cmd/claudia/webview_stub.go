//go:build !desktop

package main

import (
	"context"
	"fmt"
	"os"
)

func runDesktopWebview(want bool, panelURL string, stopRoot context.CancelFunc, rootCtx context.Context) {
	if want {
		fmt.Fprintln(os.Stderr, "claudia: desktop mode requires CGO and building with -tags desktop (try: make desktop-build)")
		os.Exit(2)
	}
	<-rootCtx.Done()
}
