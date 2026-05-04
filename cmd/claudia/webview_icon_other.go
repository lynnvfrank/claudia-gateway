//go:build desktop && !windows

package main

import webview "github.com/webview/webview_go"

func setWebviewWindowIcon(w webview.WebView) {
	_ = w
}
