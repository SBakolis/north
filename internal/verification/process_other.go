//go:build !darwin && !linux

package verification

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}
