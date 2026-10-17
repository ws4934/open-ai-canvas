//go:build !linux && !darwin

package programupdate

import (
	"errors"
	"os"
)

func lockUpdate(string) (*os.File, error) {
	return nil, errors.New("当前操作系统不支持程序文件更新")
}
