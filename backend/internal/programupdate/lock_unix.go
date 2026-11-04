//go:build linux || darwin

package programupdate

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// The service holds this lock for its lifetime, excluding both another server
// and offline repair. Process exit releases it even after a crash.
func lockUpdate(directory string) (*os.File, error) {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(directory, "update.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, errors.New("程序仍在运行或另一个更新/恢复操作持有锁，请先停止应用")
	}
	return file, nil
}
