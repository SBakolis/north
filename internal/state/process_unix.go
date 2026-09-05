//go:build darwin || linux

package state

import (
	"errors"
	"os"
	"syscall"
)

func OwnerAlive(metadata LockMetadata) bool {
	hostname, _ := os.Hostname()
	if metadata.Hostname != "" && hostname != metadata.Hostname {
		return true
	}
	if metadata.PID <= 0 {
		return false
	}
	return ProcessAlive(metadata.PID)
}

func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// ProcessTreeAlive checks both the recorded process and the process group that
// North creates for each worker, covering a surviving descendant after its
// group leader exits.
func ProcessTreeAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if ProcessAlive(pid) {
		return true
	}
	err := syscall.Kill(-pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
