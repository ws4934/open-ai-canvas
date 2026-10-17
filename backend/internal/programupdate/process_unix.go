//go:build linux || darwin

package programupdate

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func prepareCheckCommand(command *exec.Cmd) func() {
	// Candidate self-checks can spawn Node dependency imports. Cancel the owned
	// group, not only the Go parent, and bound inherited stdout pipe shutdown.
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	kill := func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	command.Cancel = kill
	command.WaitDelay = 2 * time.Second
	return func() { _ = kill() }
}
