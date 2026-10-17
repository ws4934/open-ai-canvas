//go:build !windows

package runtime

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

type ownedProcess struct{ cmd *exec.Cmd }

func prepareProcess(cmd *exec.Cmd) (*ownedProcess, error) {
	// Version-manager shims may fork Node rather than exec it. A dedicated group
	// keeps their descendants inside this run's cancellation boundary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &ownedProcess{cmd: cmd}, nil
}

func (p *ownedProcess) started() error { return nil }
func (p *ownedProcess) close()         {}

func (p *ownedProcess) kill() error {
	if p.cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
