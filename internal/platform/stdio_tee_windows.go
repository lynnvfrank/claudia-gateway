//go:build windows

package platform

import (
	"io"
	"os"
	"syscall"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
)

// hasConsoleAttached is false for PE subsystem windowsgui launched from Explorer (no conhost).
func hasConsoleAttached() bool {
	r, _, _ := procGetConsoleWindow.Call()
	return r != 0
}

// StdoutTee writes to w always; also to os.Stdout only when a console exists.
// Linking with -H=windowsgui and teeing to os.Stdout without a console can block child
// processes and break the HTTP stack; logs still go to w (e.g. servicelogs for /ui/logs).
func StdoutTee(w io.Writer) io.Writer {
	if hasConsoleAttached() {
		return io.MultiWriter(os.Stdout, w)
	}
	return w
}

// StderrTee mirrors StdoutTee for stderr.
func StderrTee(w io.Writer) io.Writer {
	if hasConsoleAttached() {
		return io.MultiWriter(os.Stderr, w)
	}
	return w
}
