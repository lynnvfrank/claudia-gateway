//go:build desktop

package main

import (
	"context"

	webview "github.com/webview/webview_go"
)

func runDesktopWebview(want bool, panelURL string, stopRoot context.CancelFunc, rootCtx context.Context) {
	if !want {
		<-rootCtx.Done()
		return
	}
	w := webview.New(false)
	defer w.Destroy()

	go func() {
		<-rootCtx.Done()
		w.Terminate()
	}()

	w.SetTitle("Claudia")
	w.SetSize(1024, 720, webview.HintNone)
	w.Navigate(panelURL)
	w.Run()
	stopRoot()
}
