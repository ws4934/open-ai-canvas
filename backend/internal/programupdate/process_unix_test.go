//go:build linux || darwin

package programupdate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProgramUpdateCheckTimeoutTerminatesDescendants(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	path := filepath.Join(dir, "candidate")
	t.Setenv("CANVAS_CHECK_TEST_CHILD_PID", pidFile)
	script := "#!/bin/sh\nsleep 60 &\nprintf '%s' \"$!\" > \"$CANVAS_CHECK_TEST_CHILD_PID\"\nwait\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := make(chan error, 1)
	finished := make(chan struct{})
	go func() { defer close(finished); _, err := runBinaryCheck(ctx, path, "--check"); result <- err }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(4 * time.Second):
			t.Error("candidate process cleanup did not finish")
		}
	})
	var pid int
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(pidFile); err == nil {
			pid, _ = strconv.Atoi(string(data))
			if pid > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pid <= 0 {
		t.Fatal("candidate did not start a descendant")
	}
	t.Cleanup(func() {
		if alive, _ := checkProcessRunning(pid); alive {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	if alive, err := checkProcessRunning(pid); err != nil || !alive {
		t.Fatal("descendant was not running before timeout", err)
	}
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("timed out candidate passed its check")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("self-check did not return within its timeout and pipe cleanup bound")
	}
	deadline = time.Now().Add(time.Second)
	for {
		alive, err := checkProcessRunning(pid)
		if err != nil {
			t.Fatal(err)
		}
		if !alive {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("self-check left a running descendant")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func checkProcessRunning(pid int) (bool, error) {
	if err := syscall.Kill(pid, 0); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return false, nil
		}
		return false, err
	}
	// An orphan already killed with its parent can remain a zombie until reaped.
	output, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return false, nil
		}
		return false, err
	}
	state := strings.TrimSpace(string(output))
	return state != "" && !strings.HasPrefix(state, "Z"), nil
}
