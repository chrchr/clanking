//go:build !windows

package daemon

import "syscall"

// detached starts the daemon in its own session so it outlives the hook that spawned it.
func detached() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }
