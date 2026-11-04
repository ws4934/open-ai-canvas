package bundled

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type entry struct {
	name, body, link string
	kind             byte
	mode             int64
}

func makeArchive(t *testing.T, entries ...entry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gzipWriter)
	for _, item := range entries {
		kind := item.kind
		if kind == 0 {
			kind = tar.TypeReg
		}
		header := &tar.Header{Name: item.name, Mode: item.mode, Typeflag: kind, Linkname: item.link}
		if kind == tar.TypeReg {
			header.Size = int64(len(item.body))
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if kind == tar.TypeReg {
			if _, err := writer.Write([]byte(item.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func runtimeArchive(t *testing.T) []byte {
	return makeArchive(t,
		entry{name: "agent-runtime/pi/agent-runtime.mjs", body: "export {};"},
		entry{name: "agent-runtime/pi/package.json", body: `{"type":"module"}`},
		entry{name: "agent-runtime/pi/node_modules/example/index.js", body: "export {};"},
		entry{name: "plugin-packages/example.yingce-plugin", body: "plugin", mode: 0o755},
	)
}

func TestUnpackRejectsUnsafeEntries(t *testing.T) {
	for _, item := range []entry{
		{name: "../escape", body: "bad"}, {name: "/tmp/escape", body: "bad"},
		{name: "agent-runtime/pi/../../../escape", body: "bad"}, {name: `agent-runtime\pi\escape`, body: "bad"},
		{name: "other/file", body: "bad"},
		{name: "agent-runtime/pi/link", kind: tar.TypeSymlink, link: "/tmp"},
		{name: "agent-runtime/pi/link", kind: tar.TypeLink, link: "agent-runtime/pi/file"},
		{name: "plugin-packages/device", kind: tar.TypeChar},
	} {
		t.Run(item.name, func(t *testing.T) {
			if err := unpack(bytes.NewReader(makeArchive(t, item)), t.TempDir()); err == nil {
				t.Fatal("unsafe entry accepted")
			}
		})
	}
	duplicate := makeArchive(t, entry{name: "plugin-packages/file", body: "a"}, entry{name: "plugin-packages/file", body: "b"})
	if err := unpack(bytes.NewReader(duplicate), t.TempDir()); err == nil {
		t.Fatal("duplicate file accepted")
	}
}

func TestUnpackChecksTrailerAndPreservesExecutables(t *testing.T) {
	data := runtimeArchive(t)
	directory := t.TempDir()
	if err := unpack(bytes.NewReader(data), directory); err != nil {
		t.Fatal(err)
	}
	if err := validateLayout(directory); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(directory, "plugin-packages/example.yingce-plugin"))
	if err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatal("executable permission lost")
	}
	data[len(data)-5] ^= 1
	if err := unpack(bytes.NewReader(data), t.TempDir()); err == nil {
		t.Fatal("corrupt gzip trailer accepted")
	}
}

func TestPrepareSelectsVersionedRuntimeWithoutChangingOnFailure(t *testing.T) {
	before := archive
	t.Cleanup(func() { archive = before })
	t.Setenv("CANVAS_PI_RUNTIME_DIR", "original-runtime")
	t.Setenv("CANVAS_OFFICIAL_PLUGIN_DIR", "original-plugins")
	archive = []byte("invalid")
	directory := t.TempDir()
	if err := Prepare(directory); err == nil {
		t.Fatal("invalid archive prepared")
	}
	if os.Getenv("CANVAS_PI_RUNTIME_DIR") != "original-runtime" {
		t.Fatal("failed extraction changed selected runtime")
	}
	archive = runtimeArchive(t)
	if err := Prepare(directory); err != nil {
		t.Fatal(err)
	}
	selected := os.Getenv("CANVAS_PI_RUNTIME_DIR")
	if !strings.HasPrefix(selected, directory+string(os.PathSeparator)) {
		t.Fatal("runtime is outside data directory")
	}
	if err := Prepare(directory); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("CANVAS_PI_RUNTIME_DIR") != selected {
		t.Fatal("same version selection is unstable")
	}
	if entries, err := os.ReadDir(filepath.Dir(filepath.Dir(selected))); err != nil || len(entries) != 2 {
		t.Fatalf("partial extraction left behind: %v %v", entries, err)
	}
}

func TestNodeVersionRequirement(t *testing.T) {
	for _, version := range []string{"v22.19.0\n", "v22.20.1", "v24.0.0"} {
		if err := validateNodeVersion(version); err != nil {
			t.Fatalf("%s: %v", version, err)
		}
	}
	for _, version := range []string{"v22.18.9", "v20.19.0", "v22.19.0-rc.1", "garbage", "v22.-1.0"} {
		if err := validateNodeVersion(version); err == nil {
			t.Fatalf("accepted %s", version)
		}
	}
}

func TestCheckValidatesImportsWithoutChangingSelectedRuntime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell")
	}
	before := archive
	archive = runtimeArchive(t)
	t.Cleanup(func() { archive = before })
	t.Setenv("CANVAS_PI_RUNTIME_DIR", "running-version")
	t.Setenv("CANVAS_OFFICIAL_PLUGIN_DIR", "running-plugins")
	t.Setenv("DATABASE_URL", "not-for-child-processes")
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := "#!/bin/sh\n[ -z \"$DATABASE_URL\" ] || exit 99\ncase \"$1\" in\n --version) echo v22.19.0;;\n --check) test -f agent-runtime.mjs;;\n --input-type=module) echo missing-dependency >&2; exit 2;;\n *) exit 3;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Check(context.Background()); err == nil || !strings.Contains(err.Error(), "check Agent runtime dependencies") {
		t.Fatalf("dependency error: %v", err)
	}
	script = strings.Replace(script, "echo missing-dependency >&2; exit 2", "exit 0", 1)
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("CANVAS_PI_RUNTIME_DIR") != "running-version" || os.Getenv("CANVAS_OFFICIAL_PLUGIN_DIR") != "running-plugins" {
		t.Fatal("candidate check selected runtime paths")
	}
}

func TestDevelopmentBundleDoesNotChangeEnvironment(t *testing.T) {
	before := archive
	archive = nil
	t.Cleanup(func() { archive = before })
	t.Setenv("CANVAS_PI_RUNTIME_DIR", "development")
	if Enabled() {
		t.Fatal("development bundle enabled")
	}
	if err := Prepare(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("CANVAS_PI_RUNTIME_DIR") != "development" {
		t.Fatal("development runtime changed")
	}
}

func TestValidatePreparedRuntimeWithoutUnpackingAgain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell")
	}
	before := archive
	archive = runtimeArchive(t)
	t.Cleanup(func() { archive = before })
	t.Setenv("CANVAS_PI_RUNTIME_DIR", "")
	t.Setenv("CANVAS_OFFICIAL_PLUGIN_DIR", "")
	t.Setenv("DATABASE_URL", "not-for-child-processes")
	if err := ValidateRuntime(context.Background()); err == nil {
		t.Fatal("unprepared runtime accepted")
	}
	if err := Prepare(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	selected := os.Getenv("CANVAS_PI_RUNTIME_DIR")
	bin := t.TempDir()
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	script := "#!/bin/sh\n[ -z \"$DATABASE_URL\" ] || exit 99\n[ -d \"$HOME\" ] || exit 98\ncase \"$1\" in\n --version) echo v22.19.0;;\n --check) test -f agent-runtime.mjs;;\n --input-type=module) exit 0;;\n *) exit 3;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	// Startup validation must use the prepared paths, not extract archive again.
	archive = []byte("not-an-archive")
	if err := ValidateRuntime(context.Background()); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("CANVAS_PI_RUNTIME_DIR") != selected {
		t.Fatal("startup validation changed the selected runtime")
	}
	script = strings.Replace(script, "echo v22.19.0", "echo v20.19.0", 1)
	if err := os.WriteFile(filepath.Join(bin, "node"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRuntime(context.Background()); err == nil || !strings.Contains(err.Error(), "Node.js >=22.19") {
		t.Fatalf("old Node version was not rejected on startup: %v", err)
	}
}
