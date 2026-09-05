//go:build !unix

package doctor

import "os/exec"

func boundCommand(*exec.Cmd) {}
