//go:build desktop && windows

package main

import (
	"os"
	"unsafe"

	"github.com/lynn/claudia-gateway/assets"
	webview "github.com/webview/webview_go"
	"golang.org/x/sys/windows"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procLoadImageW   = user32.NewProc("LoadImageW")
	procSendMessageW = user32.NewProc("SendMessageW")
)

const (
	imageIcon      = 1
	lrLoadFromFile = 0x10
	lrDefaultSize  = 0x40
	wmSetIcon      = 0x80
	iconSmall      = 0
	iconBig        = 1
)

func setWebviewWindowIcon(w webview.WebView) {
	if len(assets.IconICO) == 0 {
		return
	}
	f, err := os.CreateTemp("", "claudia-*.ico")
	if err != nil {
		return
	}
	path := f.Name()
	if _, err := f.Write(assets.IconICO); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return
	}
	defer func() { _ = os.Remove(path) }()

	pathW, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	hIcon, _, _ := procLoadImageW.Call(
		0,
		uintptr(unsafe.Pointer(pathW)),
		uintptr(imageIcon),
		0,
		0,
		uintptr(lrLoadFromFile|lrDefaultSize),
	)
	if hIcon == 0 {
		return
	}
	hwnd := uintptr(w.Window())
	if hwnd == 0 {
		return
	}
	procSendMessageW.Call(hwnd, uintptr(wmSetIcon), uintptr(iconSmall), hIcon)
	procSendMessageW.Call(hwnd, uintptr(wmSetIcon), uintptr(iconBig), hIcon)
}
