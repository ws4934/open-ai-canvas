//go:build !windows

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunTerminatesWrapperDescendants(t *testing.T) {
	lookupCtx, cancelLookup := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelLookup()
	output, err := exec.CommandContext(lookupCtx, "node", "-p", "process.execPath").Output()
	if err != nil {
		t.Fatalf("resolve Node executable: %v", err)
	}
	realNode := strings.TrimSpace(string(output))
	for _, scenario := range []string{"cancel", "oversized-output"} {
		t.Run(scenario, func(t *testing.T) {
			directory := t.TempDir()
			pidFile := filepath.Join(directory, "child.pid")
			script := fmt.Sprintf("import { writeFileSync } from 'node:fs'; writeFileSync(%q, String(process.pid));", pidFile)
			if scenario == "oversized-output" {
				script += `process.stdout.write("x".repeat(8 * 1024 * 1024));`
			}
			script += `setInterval(() => {}, 1000);`
			fakeRuntime(t, script)
			wrapper := "#!/bin/sh\n'" + strings.ReplaceAll(realNode, "'", "'\\''") + "' \"$@\" &\nwait\n"
			if err := os.WriteFile(filepath.Join(directory, "node"), []byte(wrapper), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- Run(ctx, ProcessRequest{}, noopBridge()) }()
			var pid int
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				data, readErr := os.ReadFile(pidFile)
				if readErr == nil {
					pid, err = strconv.Atoi(string(data))
					if err == nil && pid > 0 {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if pid == 0 {
				cancel()
				t.Fatal("wrapper descendant did not start")
			}
			t.Cleanup(func() {
				if processRunning(pid) {
					_ = syscall.Kill(pid, syscall.SIGKILL)
				}
			})
			if scenario == "cancel" {
				cancel()
			}
			select {
			case err := <-result:
				if scenario == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel result: %v", err)
				}
				if scenario == "oversized-output" && (err == nil || !strings.Contains(err.Error(), "read Agent runtime output")) {
					t.Fatalf("output result: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Run remained blocked on a wrapper descendant's stdout")
			}
			deadline = time.Now().Add(time.Second)
			for processRunning(pid) && time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
			}
			if processRunning(pid) {
				t.Fatal("Node descendant remained alive")
			}
			if active, _ := ProcessUsage(); active != 0 {
				t.Fatalf("process capacity leaked: %d", active)
			}
		})
	}
}

func processRunning(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	// A killed orphan can briefly remain a zombie until the OS reaps it.
	output, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	return err == nil && strings.TrimSpace(string(output)) != "" && !strings.HasPrefix(strings.TrimSpace(string(output)), "Z")
}
