package programupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"yingce/backend/internal/buildinfo"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func testManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "system", "bin", "yingce-backend")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("v1.0.0.0"), 0o700); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(Config{DataDir: dir, ManagedBinaryPath: path, CurrentVersion: "v1.0.0.0", RequestRestart: func() error { return nil }, Drain: func(context.Context) error { return nil }, Resume: func() {}})
	if err != nil {
		t.Fatal(err)
	}
	m.supported, m.unsupportedReason = true, ""
	t.Cleanup(func() { _ = m.Close() })
	m.arch = "amd64"
	m.checkBinary = func(ctx context.Context, path, mode string) (buildinfo.Info, error) {
		data, err := os.ReadFile(path)
		return buildinfo.Info{Version: string(data)}, err
	}
	return m
}

func archiveBytes(t *testing.T, entries ...*tar.Header) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		if err := tw.WriteHeader(entry); err != nil {
			t.Fatal(err)
		}
		if entry.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte("v1.0.0.1")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func releaseClient(t *testing.T, m *Manager, corrupt bool) *atomic.Int32 {
	t.Helper()
	archive := archiveBytes(t, &tar.Header{Name: "yingce-backend", Mode: 0o755, Size: 8, Typeflag: tar.TypeReg})
	hash := sha256.Sum256(archive)
	sums := hex.EncodeToString(hash[:]) + "  open-ai-canvas_v1.0.0.1_linux_amd64.tar.gz\n"
	if corrupt {
		archive = append(archive, 0)
	}
	base := "https://github.com/" + Repository + "/releases/download/v1.0.0.1/"
	var calls atomic.Int32
	m.client = &http.Client{Transport: testTransport(func(req *http.Request) (*http.Response, error) {
		var body []byte
		switch {
		case req.URL.Host == "api.github.com":
			calls.Add(1)
			body, _ = json.Marshal([]map[string]any{
				{"tag_name": "v9.0.0.0", "prerelease": true}, {"tag_name": "v8.0.0.0", "draft": true},
				{"tag_name": "v1.0.0.9", "assets": []asset{{Name: "release-notes.txt"}, {Name: "SHA256SUMS"}}},
				{"tag_name": "v1.0.0.1", "name": "stable", "assets": []asset{{Name: "SHA256SUMS", URL: base + "SHA256SUMS", Size: int64(len(sums))}, {Name: "open-ai-canvas_v1.0.0.1_linux_amd64.tar.gz", URL: base + "open-ai-canvas_v1.0.0.1_linux_amd64.tar.gz", Size: int64(len(archive))}}},
			})
		case strings.HasSuffix(req.URL.Path, "SHA256SUMS"):
			body = []byte(sums)
		default:
			body = archive
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
	})}
	return &calls
}

func waitOperation(t *testing.T, m *Manager, phase Phase) Status {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		status, _ := m.Status(context.Background())
		if status.Operation.Phase == phase {
			return status
		}
		select {
		case <-deadline:
			t.Fatalf("wanted %s, got %#v", phase, status.Operation)
		case <-time.After(time.Millisecond):
		}
	}
}

func TestProgramUpdateVersionAndStableCache(t *testing.T) {
	for _, version := range []string{"v1.2.3", "1.2.3.12"} {
		if !IsReleaseVersion(version) {
			t.Fatal(version)
		}
	}
	for _, version := range []string{"v1.2.3-beta", "v1.2.3+meta", "1.2", "1.2.3.4.5", "1.2.-3", "1.02.3"} {
		if IsReleaseVersion(version) {
			t.Fatal(version)
		}
	}
	if CompareVersions("v1.2.3.12", "v1.2.3.9") != 1 || CompareVersions("1.2.3", "1.2.3.0") != 0 {
		t.Fatal("numeric version comparison")
	}
	m := testManager(t)
	calls := releaseClient(t, m, false)
	for i := 0; i < 2; i++ {
		status, err := m.Check(context.Background(), false)
		if err != nil || status.LatestRelease.Version != "v1.0.0.1" {
			t.Fatalf("status=%+v err=%v", status, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("automatic checks bypassed cache")
	}
	if _, err := m.Check(context.Background(), true); err != nil || calls.Load() != 2 {
		t.Fatal("manual refresh did not force check", err)
	}
}

func TestProgramUpdateAsyncSwapAndConfirm(t *testing.T) {
	m := testManager(t)
	releaseClient(t, m, false)
	draining := make(chan struct{})
	allow := make(chan struct{})
	restarted := make(chan struct{})
	m.config.Drain = func(context.Context) error { close(draining); <-allow; return nil }
	m.config.RequestRestart = func() error { close(restarted); return nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	first, err := m.Start(ctx, "v1.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	<-draining
	duplicate, err := m.Start(context.Background(), "v1.0.0.1")
	if err != nil || duplicate.Operation.ID != first.Operation.ID {
		t.Fatal("duplicate did not reuse operation", err)
	}
	if _, err := m.Start(context.Background(), "v1.0.0.2"); err == nil {
		t.Fatal("concurrent different update accepted")
	}
	close(allow)
	<-restarted
	current, _ := os.ReadFile(m.config.ManagedBinaryPath)
	backup, _ := os.ReadFile(m.config.ManagedBinaryPath + ".backup")
	if string(current) != "v1.0.0.1" || string(backup) != "v1.0.0.0" {
		t.Fatalf("current=%s backup=%s", current, backup)
	}
	config := m.config
	config.CurrentVersion = "v1.0.0.1"
	next, err := NewManager(config)
	if err != nil {
		t.Fatal(err)
	}
	next.supported = true
	if err := next.ConfirmStarted(); err != nil {
		t.Fatal(err)
	}
	status, _ := next.Status(context.Background())
	if status.Operation.Phase != PhaseSucceeded || status.RollbackVersion != "v1.0.0.0" {
		t.Fatalf("%+v", status)
	}
}

func TestProgramUpdateBadChecksumAndDrainFailureKeepCurrent(t *testing.T) {
	for _, corrupt := range []bool{true, false} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			m := testManager(t)
			releaseClient(t, m, corrupt)
			var drains, resumes atomic.Int32
			m.config.Drain = func(context.Context) error { drains.Add(1); return errors.New("pending approval") }
			m.config.Resume = func() { resumes.Add(1) }
			if _, err := m.Start(context.Background(), "v1.0.0.1"); err != nil {
				t.Fatal(err)
			}
			waitOperation(t, m, PhaseFailed)
			current, _ := os.ReadFile(m.config.ManagedBinaryPath)
			if string(current) != "v1.0.0.0" {
				t.Fatal("failed update replaced current")
			}
			if corrupt && drains.Load() != 0 {
				t.Fatal("invalid archive reached drain")
			}
			if !corrupt && resumes.Load() != 1 {
				t.Fatal("failed drain did not resume intake")
			}
		})
	}
}

func TestProgramUpdateArchiveAndDownloadTrust(t *testing.T) {
	for _, raw := range []string{"http://github.com/a", "https://github.com.evil/a", "https://user@github.com/a", "https://github.com:444/a", "https://127.0.0.1/a"} {
		if safeURL(raw) {
			t.Fatal(raw)
		}
	}
	client := newHTTPClient()
	uri, _ := url.Parse("https://127.0.0.1/private")
	if err := client.CheckRedirect(&http.Request{URL: uri}, nil); err == nil {
		t.Fatal("unsafe redirect accepted")
	}
	for _, entry := range []*tar.Header{
		{Name: "../yingce-backend", Typeflag: tar.TypeReg, Size: 8},
		{Name: "yingce-backend", Typeflag: tar.TypeSymlink, Linkname: "/tmp/executable"},
		{Name: "/yingce-backend", Typeflag: tar.TypeReg, Size: 8},
		{Name: "other", Typeflag: tar.TypeReg, Size: 8},
	} {
		if err := extractBinary(bytes.NewReader(archiveBytes(t, entry)), io.Discard); err == nil {
			t.Fatalf("accepted %s", entry.Name)
		}
	}
	entry := &tar.Header{Name: "yingce-backend", Typeflag: tar.TypeReg, Size: 8}
	if err := extractBinary(bytes.NewReader(archiveBytes(t, entry, entry)), io.Discard); err == nil {
		t.Fatal("duplicate binary accepted")
	}
}

func TestProgramUpdateSwapFailureRestoresSource(t *testing.T) {
	m := testManager(t)
	candidate := filepath.Join(m.binDir(), "candidate")
	if err := os.WriteFile(candidate, []byte("target"), 0o700); err != nil {
		t.Fatal(err)
	}
	count := 0
	err := swapProgramWithRename(m.config.ManagedBinaryPath, candidate, func(from, to string) error {
		count++
		if count == 2 {
			return errors.New("simulated rename failure")
		}
		return os.Rename(from, to)
	})
	if err == nil {
		t.Fatal("missing swap failure")
	}
	data, _ := os.ReadFile(m.config.ManagedBinaryPath)
	if string(data) != "v1.0.0.0" {
		t.Fatal("source not restored")
	}
}

func TestProgramUpdateRollbackSchemaAndStartupFailure(t *testing.T) {
	m := testManager(t)
	if err := os.WriteFile(m.config.ManagedBinaryPath+".backup", []byte("v0.9.0.0"), 0o700); err != nil {
		t.Fatal(err)
	}
	m.state.RollbackVersion = "v0.9.0.0"
	m.checkBinary = func(context.Context, string, string) (buildinfo.Info, error) {
		return buildinfo.Info{}, errors.New("schema mismatch")
	}
	if _, err := m.Rollback(context.Background(), "恢复旧版本"); err != nil {
		t.Fatal(err)
	}
	status := waitOperation(t, m, PhaseFailed)
	if status.Operation.Error == "" {
		t.Fatal("rollback failure not recorded")
	}
	data, _ := os.ReadFile(m.config.ManagedBinaryPath)
	if string(data) != "v1.0.0.0" {
		t.Fatal("incompatible rollback swapped")
	}
	m.mu.Lock()
	m.state.Operation = Operation{Phase: PhaseRestarting, TargetVersion: "v1.0.0.0", FromVersion: "v0.9.0.0", Logs: []LogEntry{}}
	m.mu.Unlock()
	if err := m.FailStartup(errors.New("startup failed")); err == nil {
		t.Fatal("schema-incompatible startup rollback accepted")
	}
	status, _ = m.Status(context.Background())
	if status.Operation.Phase != PhaseManualIntervention {
		t.Fatal(status.Operation)
	}
	loaded, err := NewManager(m.config)
	if err != nil {
		t.Fatal(err)
	}
	status, _ = loaded.Status(context.Background())
	if status.Operation.Phase != PhaseManualIntervention {
		t.Fatal("failure not persisted")
	}
	if err := loaded.FailStartup(errors.New("ordinary startup failure")); err != nil {
		t.Fatal(err)
	}
}

func TestProgramUpdateUnsupportedCanCheckButNotReplace(t *testing.T) {
	m := testManager(t)
	m.supported = false
	m.unsupportedReason = "development"
	releaseClient(t, m, false)
	if _, err := m.Check(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(context.Background(), "v1.0.0.1"); err == nil {
		t.Fatal("unmanaged update allowed")
	}
}

func TestProgramUpdateSourceShutdownFailureDoesNotRewritePending(t *testing.T) {
	m := testManager(t)
	m.state.Operation = Operation{Phase: PhaseRestarting, FromVersion: "v1.0.0.0", TargetVersion: "v1.0.0.1", Logs: []LogEntry{}}
	if err := m.FailStartup(errors.New("source HTTP shutdown timed out")); err != nil {
		t.Fatal(err)
	}
	status, _ := m.Status(context.Background())
	if status.Operation.Phase != PhaseRestarting || status.Operation.TargetVersion != "v1.0.0.1" {
		t.Fatal("source shutdown overwrote pending startup", status.Operation)
	}
}

func TestProgramUpdateStartupRecoveryRunsOnlyOnce(t *testing.T) {
	m := testManager(t)
	if err := os.WriteFile(m.config.ManagedBinaryPath+".backup", []byte("v0.9.0.0"), 0o700); err != nil {
		t.Fatal(err)
	}
	m.state.Operation = Operation{Phase: PhaseRestarting, FromVersion: "v0.9.0.0", TargetVersion: "v1.0.0.0", Logs: []LogEntry{}}
	if err := m.FailStartup(errors.New("candidate startup failure")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(m.config.ManagedBinaryPath)
	if string(data) != "v0.9.0.0" || !m.state.Operation.AutomaticRollback {
		t.Fatal("compatible source not restored")
	}
	m.config.CurrentVersion = "v0.9.0.0"
	if err := m.FailStartup(errors.New("source also failed")); err != nil {
		t.Fatal(err)
	}
	status, _ := m.Status(context.Background())
	if status.Operation.Phase != PhaseManualIntervention || status.RollbackVersion != "v1.0.0.0" {
		t.Fatal(status)
	}
	data, _ = os.ReadFile(m.config.ManagedBinaryPath)
	if string(data) != "v0.9.0.0" {
		t.Fatal("recovery loop switched to failed candidate")
	}
}

func TestProgramUpdateOfflineRestoreRequiresExclusiveLock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CANVAS_BACKEND_DATA_DIR", dir)
	path := filepath.Join(dir, "system", "bin", "yingce-backend")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	script := func(version string) []byte {
		return []byte("#!/bin/sh\nprintf '%s\\n' '{\"version\":\"" + version + "\"}'\n")
	}
	if err := os.WriteFile(path, script("v1.0.0.1"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".backup", script("v1.0.0.0"), 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := lockUpdate(filepath.Join(dir, "system"))
	if err != nil {
		t.Fatal(err)
	}
	if err := RestoreProgram(context.Background(), path, dir); err == nil {
		t.Fatal("restored while service held lock")
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if err := RestoreProgram(context.Background(), path, dir); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "v1.0.0.0") {
		t.Fatal("offline restore failed")
	}
	var state persistedState
	data, err = os.ReadFile(filepath.Join(dir, "system", "update.json"))
	if err != nil || json.Unmarshal(data, &state) != nil {
		t.Fatal("missing offline recovery state", err)
	}
	if state.Operation.TargetVersion != "v1.0.0.0" || state.Operation.FromVersion != "v1.0.0.1" || state.RollbackVersion != "v1.0.0.1" {
		t.Fatal(state)
	}
}

func TestProgramUpdateStateWriteFailureDoesNotStrandOperation(t *testing.T) {
	m := testManager(t)
	if err := os.Mkdir(m.statePath(), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(context.Background(), "v1.0.0.1"); err == nil {
		t.Fatal("accepted unpersisted operation")
	}
	status, _ := m.Status(context.Background())
	if status.Operation.Phase != PhaseIdle {
		t.Fatal("failed state write left active operation")
	}
}

func TestProgramUpdateCustomBinaryUsesSameDefaultDataLock(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("CANVAS_BACKEND_DATA_DIR", "")
	if err := os.Unsetenv("CANVAS_BACKEND_DATA_DIR"); err != nil {
		t.Fatal(err)
	}
	managedPath := filepath.Join(dir, "custom-programs", "bin", "yingce-backend")
	if err := os.MkdirAll(filepath.Dir(managedPath), 0o700); err != nil {
		t.Fatal(err)
	}
	script := func(version string) []byte {
		return []byte("#!/bin/sh\nprintf '%s\\n' '{\"version\":\"" + version + "\"}'\n")
	}
	if err := os.WriteFile(managedPath, script("v1.0.0.1"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(managedPath+".backup", script("v1.0.0.0"), 0o700); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(Config{ManagedBinaryPath: managedPath, CurrentVersion: "v1.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	// Test binaries are not the managed executable; acquire the same lifetime
	// lock as a supported Manager without changing the current test executable.
	m.lockFile, err = lockUpdate(filepath.Dir(m.statePath()))
	if err != nil {
		t.Fatal(err)
	}
	if err := RestoreProgram(context.Background(), managedPath, ""); err == nil {
		t.Fatal("custom binary path bypassed running Manager's default data lock")
	}
	data, _ := os.ReadFile(managedPath)
	if !strings.Contains(string(data), "v1.0.0.1") {
		t.Fatal("live program replaced")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if err := RestoreProgram(context.Background(), managedPath, "data"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(m.statePath()); err != nil {
		t.Fatal("restore did not use Manager state path", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "custom-programs", "update.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("restore derived state path from binary path", err)
	}
}

func TestProgramUpdateCheckFailureProvidesSafeRuntimeGuidance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate")
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho 'secret://database-password' >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"--check", "--check-schema"} {
		_, err := runBinaryCheck(context.Background(), path, mode)
		if err == nil || strings.Contains(err.Error(), "database-password") {
			t.Fatal("missing or unsafe error", err)
		}
		if mode == "--check" && !strings.Contains(err.Error(), "官方 Docker 运行环境镜像") {
			t.Fatal("missing runtime image guidance", err)
		}
		if mode == "--check-schema" && !strings.Contains(err.Error(), "数据库结构") {
			t.Fatal("missing schema compatibility guidance", err)
		}
	}
}
