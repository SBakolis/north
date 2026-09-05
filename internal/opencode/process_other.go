//go:build !darwin && !linux

package opencode

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}
