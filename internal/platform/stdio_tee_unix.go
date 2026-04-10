//go:build !windows

package platform

import (
	"io"
	"os"
)

// StdoutTee writes to w and to os.Stdout (supervised logs + operator console).
func StdoutTee(w io.Writer) io.Writer {
	return io.MultiWriter(os.Stdout, w)
}

// StderrTee writes to w and to os.Stderr.
func StderrTee(w io.Writer) io.Writer {
	return io.MultiWriter(os.Stderr, w)
}
