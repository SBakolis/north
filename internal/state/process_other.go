//go:build !darwin && !linux

package state

// OwnerAlive is conservative where portable process probing is unavailable.
func OwnerAlive(metadata LockMetadata) bool { return true }

func ProcessAlive(pid int) bool     { return pid > 0 }
func ProcessTreeAlive(pid int) bool { return pid > 0 }
