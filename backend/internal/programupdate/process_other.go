//go:build !linux && !darwin

package programupdate

import (
	"os/exec"
	"time"
)

// Managed program replacement is unsupported here; retain bounded standard
// command cleanup for callers compiling the package on other platforms.
func prepareCheckCommand(command *exec.Cmd) func() {
	command.WaitDelay = 2 * time.Second
	return func() {}
}
