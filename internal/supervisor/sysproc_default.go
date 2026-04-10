//go:build !windows

package supervisor

import "os/exec"

func applyNoConsoleWindow(cmd *exec.Cmd) {}
