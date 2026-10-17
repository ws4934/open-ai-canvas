package bundled

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"yingce/backend/internal/buildinfo"

	"github.com/gofrs/flock"
)

func Enabled() bool { return len(archive) > 0 }

// Prepare makes the current executable's runtime available independently of its
// working directory. A complete version is renamed into place before selection.
func Prepare(dataDir string) error {
	if !Enabled() {
		return nil
	}
	info := buildinfo.Current()
	identity := base64.RawURLEncoding.EncodeToString([]byte(info.Version + "\n" + info.Commit + "\n" + info.BuildTime))
	directory, err := filepath.Abs(filepath.Join(dataDir, "runtime", "bundled"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(directory, ".lock"))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()
	target := filepath.Join(directory, identity)
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		stage, err := os.MkdirTemp(directory, ".stage-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(stage)
		if err := unpack(bytes.NewReader(archive), stage); err != nil {
			return err
		}
		if err := validateLayout(stage); err != nil {
			return err
		}
		if err := os.Rename(stage, target); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if err := validateLayout(target); err != nil {
		return err
	}
	if err := os.Setenv("CANVAS_PI_RUNTIME_DIR", filepath.Join(target, "agent-runtime", "pi")); err != nil {
		return err
	}
	return os.Setenv("CANVAS_OFFICIAL_PLUGIN_DIR", filepath.Join(target, "plugin-packages"))
}

// Check verifies a downloaded executable without opening application databases,
// changing configured runtime paths, or starting an Agent session.
func Check(ctx context.Context) error {
	if !Enabled() {
		return nil
	}
	directory, err := os.MkdirTemp("", "canvas-runtime-check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	if err := unpack(bytes.NewReader(archive), directory); err != nil {
		return err
	}
	if err := validateLayout(directory); err != nil {
		return err
	}
	return checkNode(ctx, filepath.Join(directory, "agent-runtime", "pi"), directory)
}

// ValidateRuntime checks the already prepared runtime on startup. It does not
// unpack the executable again or give dependency imports access to user data.
func ValidateRuntime(ctx context.Context) error {
	if !Enabled() {
		return nil
	}
	runtimeDir := strings.TrimSpace(os.Getenv("CANVAS_PI_RUNTIME_DIR"))
	if runtimeDir == "" {
		return errors.New("bundled runtime has not been prepared")
	}
	home, err := os.MkdirTemp("", "canvas-runtime-validation-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(home)
	return checkNode(ctx, runtimeDir, home)
}

func validateLayout(directory string) error {
	for _, name := range []string{"agent-runtime/pi/agent-runtime.mjs", "agent-runtime/pi/package.json"} {
		info, err := os.Lstat(filepath.Join(directory, filepath.FromSlash(name)))
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("bundled runtime is missing regular file %s", name)
		}
	}
	for _, name := range []string{"agent-runtime/pi/node_modules", "plugin-packages"} {
		info, err := os.Lstat(filepath.Join(directory, filepath.FromSlash(name)))
		if err != nil || !info.IsDir() {
			return fmt.Errorf("bundled runtime is missing directory %s", name)
		}
		entries, err := os.ReadDir(filepath.Join(directory, filepath.FromSlash(name)))
		if err != nil || len(entries) == 0 {
			return fmt.Errorf("bundled runtime directory %s is empty or unreadable", name)
		}
	}
	return nil
}

func checkNode(ctx context.Context, runtimeDir, home string) error {
	binary, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("Node.js >=22.19 is required: %w", err)
	}
	run := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = runtimeDir
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "NODE_ENV=production", "PI_OFFLINE=1"}
		return cmd.CombinedOutput()
	}
	version, err := run("--version")
	if err != nil {
		return fmt.Errorf("read Node.js version: %w", err)
	}
	if err := validateNodeVersion(string(version)); err != nil {
		return err
	}
	if output, err := run("--check", "agent-runtime.mjs"); err != nil {
		return fmt.Errorf("check Agent runtime syntax: %w: %s", err, strings.TrimSpace(string(output)))
	}
	const imports = `await Promise.all(["undici", "@earendil-works/pi-coding-agent", "@earendil-works/pi-ai", "typebox"].map(name => import(name)));`
	if output, err := run("--input-type=module", "--eval", imports); err != nil {
		return fmt.Errorf("check Agent runtime dependencies: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func validateNodeVersion(value string) error {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(value), "v"), ".")
	if len(parts) != 3 {
		return errors.New("cannot determine Node.js version")
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	patch, patchErr := strconv.Atoi(parts[2])
	if majorErr != nil || minorErr != nil || patchErr != nil || patch < 0 || minor < 0 || major < 22 || major == 22 && minor < 19 {
		return fmt.Errorf("Node.js >=22.19 is required; found %s", strings.TrimSpace(value))
	}
	return nil
}
