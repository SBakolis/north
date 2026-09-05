//go:build unix

package doctor

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func boundCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}
}
