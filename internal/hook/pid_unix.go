//go:build !windows

package hook

import "syscall"

func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func killPID(pid int) { _ = syscall.Kill(pid, syscall.SIGTERM) }
