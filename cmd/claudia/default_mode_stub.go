//go:build !desktop

package main

func defaultNoSubcommandUsesDesktopUI() bool {
	return false
}
