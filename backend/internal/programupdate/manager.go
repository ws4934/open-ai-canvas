package programupdate

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"yingce/backend/internal/buildinfo"
)

type persistedState struct {
	FormatVersion           int       `json:"formatVersion"`
	RollbackVersion         string    `json:"rollbackVersion,omitempty"`
	PreviousRollbackVersion string    `json:"previousRollbackVersion,omitempty"`
	Operation               Operation `json:"operation"`
}

type Manager struct {
	mu                sync.Mutex
	checkMu           sync.Mutex
	config            Config
	state             persistedState
	client            *http.Client
	arch              string
	supported         bool
	unsupportedReason string
	latest            *Release
	checkedAt         *time.Time
	checkError        error
	running           bool
	checkBinary       func(context.Context, string, string) (buildinfo.Info, error)
	lifetime          context.Context
	cancel            context.CancelFunc
	operations        sync.WaitGroup
	lockFile          *os.File
}

func NewManager(config Config) (*Manager, error) {
	if config.CurrentVersion == "" {
		config.CurrentVersion = config.BuildInfo.Version
	}
	if config.CurrentVersion == "" {
		config.CurrentVersion = buildinfo.Current().Version
	}
	if config.DataDir == "" {
		config.DataDir = "data"
	}
	if config.ManagedBinaryPath != "" {
		absolute, err := filepath.Abs(config.ManagedBinaryPath)
		if err != nil {
			return nil, err
		}
		config.ManagedBinaryPath = absolute
	}
	m := &Manager{config: config, client: newHTTPClient(), arch: runtime.GOARCH, checkBinary: runBinaryCheck,
		state: persistedState{FormatVersion: 1, Operation: Operation{Phase: PhaseIdle, Logs: []LogEntry{}}}}
	m.lifetime, m.cancel = context.WithCancel(context.Background())
	initialized := false
	defer func() {
		if !initialized {
			_ = m.Close()
		}
	}()
	m.supported, m.unsupportedReason = managedExecutable(config.ManagedBinaryPath)
	if m.supported {
		var lockErr error
		m.lockFile, lockErr = lockUpdate(filepath.Dir(m.statePath()))
		if lockErr != nil {
			return nil, lockErr
		}
	}
	data, err := os.ReadFile(m.statePath())
	if err == nil {
		if len(data) > 1<<20 || json.Unmarshal(data, &m.state) != nil || m.state.FormatVersion != 1 {
			return nil, errors.New("程序更新状态损坏，请保留状态文件并人工检查")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取程序更新状态：%w", err)
	}
	if m.state.Operation.Logs == nil {
		m.state.Operation.Logs = []LogEntry{}
	}
	if m.supported {
		if err := os.MkdirAll(m.binDir(), 0o700); err != nil {
			return nil, err
		}
		if m.state.Operation.Phase.Active() && m.state.Operation.Phase != PhaseSwitching && m.state.Operation.Phase != PhaseRestarting {
			m.finishLocked(PhaseFailed, "更新进程在替换程序前中断，当前程序保持不变")
			if err := m.saveLocked(); err != nil {
				return nil, err
			}
		}
	}
	initialized = true
	return m, nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.cancel()
	m.mu.Unlock()
	m.operations.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lockFile == nil {
		return nil
	}
	err := m.lockFile.Close()
	m.lockFile = nil
	return err
}

func managedExecutable(path string) (bool, string) {
	if runtime.GOOS != "linux" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return false, "网页更新仅支持 Linux amd64/arm64 官方程序"
	}
	if path == "" {
		return false, "当前未使用受管程序路径，可检查版本但不能网页替换程序"
	}
	executable, err := os.Executable()
	if err != nil {
		return false, "无法确认正在运行的程序路径"
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return false, "无法确认正在运行的程序路径"
	}
	managed, err := filepath.EvalSymlinks(path)
	if err != nil || executable != managed {
		return false, "当前程序不在受管路径，禁止替换其他程序"
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false, "受管程序必须是普通文件"
	}
	return true, ""
}

func updateStatePath(dataDir string) string {
	if dataDir == "" {
		dataDir = "data"
	}
	return filepath.Join(dataDir, "system", "update.json")
}

func (m *Manager) statePath() string { return updateStatePath(m.config.DataDir) }
func (m *Manager) binDir() string    { return filepath.Dir(m.config.ManagedBinaryPath) }

func (m *Manager) snapshotLocked() Status {
	op := m.state.Operation
	op.Logs = append([]LogEntry{}, op.Logs...)
	status := Status{Supported: m.supported, UnsupportedReason: m.unsupportedReason, Repository: Repository,
		CurrentVersion: m.config.CurrentVersion, LatestRelease: m.latest, CheckedAt: m.checkedAt,
		RollbackVersion: m.state.RollbackVersion, Operation: op}
	status.UpdateAvailable = m.latest != nil && CompareVersions(m.latest.Version, m.config.CurrentVersion) > 0
	if m.supported && m.state.RollbackVersion != "" && CompareVersions(m.state.RollbackVersion, m.config.CurrentVersion) < 0 && !op.Phase.Active() {
		info, err := os.Lstat(m.config.ManagedBinaryPath + ".backup")
		status.CanRollback = err == nil && info.Mode().IsRegular()
	}
	return status
}

func (m *Manager) Status(context.Context) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked(), nil
}

func (m *Manager) Check(ctx context.Context, force bool) (Status, error) {
	m.checkMu.Lock()
	defer m.checkMu.Unlock()
	m.mu.Lock()
	if !force && m.checkedAt != nil && time.Since(*m.checkedAt) < checkCacheTTL {
		status := m.snapshotLocked()
		checkErr := m.checkError
		m.mu.Unlock()
		return status, checkErr
	}
	m.mu.Unlock()
	releases, err := m.fetchReleases(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		return m.snapshotLocked(), err
	}
	m.latest = nil
	m.checkError = nil
	for i := range releases {
		if _, _, err := m.installableAssets(&releases[i]); err != nil {
			continue
		}
		if m.latest == nil || CompareVersions(releases[i].Version, m.latest.Version) > 0 {
			copy := releases[i]
			m.latest = &copy
		}
	}
	now := time.Now().UTC()
	m.checkedAt = &now
	if m.latest == nil {
		m.checkError = ErrNoInstallableRelease
	}
	return m.snapshotLocked(), m.checkError
}

func (m *Manager) Start(ctx context.Context, target string) (Status, error) {
	if !IsReleaseVersion(target) {
		status, _ := m.Status(ctx)
		return status, errors.New("目标必须是正式三段或四段数字版本")
	}
	target = releaseTag(target)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.beginLocked("update", target); err != nil {
		return m.snapshotLocked(), err
	}
	if m.state.Operation.TargetVersion == target && m.state.Operation.Phase != PhaseDownloading {
		return m.snapshotLocked(), nil
	}
	// beginLocked rejects different targets and marks a duplicate without starting work again.
	if m.running {
		return m.snapshotLocked(), nil
	}
	m.running = true
	m.operations.Add(1)
	go m.runUpdate(target)
	return m.snapshotLocked(), nil
}

func (m *Manager) beginLocked(kind, target string) error {
	if m.lifetime.Err() != nil {
		return errors.New("程序正在停止，不能开始更新")
	}
	if !m.supported {
		return errors.New(m.unsupportedReason)
	}
	if m.config.RequestRestart == nil || m.config.Drain == nil || m.config.Resume == nil {
		return errors.New("更新重启与排空控制尚未就绪")
	}
	if m.state.Operation.Phase.Active() {
		if m.state.Operation.Kind == kind && m.state.Operation.TargetVersion == target {
			return nil
		}
		return errors.New("已有程序更新或回退正在执行")
	}
	if m.running {
		return errors.New("上一次更新操作仍在收尾，请稍后重试")
	}
	if kind == "update" && CompareVersions(target, m.config.CurrentVersion) <= 0 {
		return errors.New("目标版本必须高于当前版本")
	}
	if kind == "rollback" && CompareVersions(target, m.config.CurrentVersion) >= 0 {
		return errors.New("程序备份不是较早版本，请通过更新重新安装新版本")
	}
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	now := time.Now().UTC()
	previous := m.state
	m.state.Operation = Operation{ID: hex.EncodeToString(random), Kind: kind, Phase: PhaseDownloading,
		FromVersion: m.config.CurrentVersion, TargetVersion: target, StartedAt: &now, Logs: []LogEntry{}}
	m.logLocked("准备" + map[string]string{"update": "更新", "rollback": "回退"}[kind] + "程序：" + m.config.CurrentVersion + " → " + target)
	if err := m.saveLocked(); err != nil {
		m.state = previous
		return err
	}
	return nil
}

func (m *Manager) runUpdate(target string) {
	defer m.operations.Done()
	ctx, cancel := context.WithTimeout(m.lifetime, 30*time.Minute)
	defer cancel()
	defer func() { m.mu.Lock(); m.running = false; m.mu.Unlock() }()
	release, err := m.releaseFor(ctx, target)
	if err != nil {
		m.fail(err)
		return
	}
	candidate, err := m.downloadCandidate(ctx, release)
	if err != nil {
		m.fail(err)
		return
	}
	defer os.Remove(candidate)
	if err := m.phase(PhaseValidating, "校验候选程序与运行环境"); err != nil {
		m.fail(err)
		return
	}
	info, err := m.checkBinary(ctx, candidate, "--check")
	if err != nil {
		m.fail(err)
		return
	}
	if releaseTag(info.Version) != target {
		m.fail(errors.New("候选程序版本与 Release 不一致"))
		return
	}
	m.activate(ctx, candidate)
}

func (m *Manager) activate(ctx context.Context, candidate string) {
	if err := m.phase(PhaseDraining, "暂停接收新任务，等待已接任务和审批结束"); err != nil {
		m.fail(err)
		return
	}
	drainCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	err := m.config.Drain(drainCtx)
	cancel()
	if err != nil {
		m.config.Resume()
		m.fail(fmt.Errorf("任务排空未完成：%w", err))
		return
	}
	m.mu.Lock()
	m.state.Operation.Phase = PhaseSwitching
	m.logLocked("替换程序文件，保留上一版本")
	previousBackup := m.state.RollbackVersion
	m.state.PreviousRollbackVersion = previousBackup
	m.state.RollbackVersion = m.state.Operation.FromVersion
	err = m.saveLocked()
	m.mu.Unlock()
	if err != nil {
		m.config.Resume()
		m.fail(err)
		return
	}
	if err := swapProgram(m.config.ManagedBinaryPath, candidate); err != nil {
		m.mu.Lock()
		m.state.RollbackVersion = previousBackup
		m.mu.Unlock()
		var unsafe *unsafeSwapError
		if errors.As(err, &unsafe) {
			m.mu.Lock()
			m.finishLocked(PhaseManualIntervention, err.Error())
			_ = m.saveLocked()
			m.mu.Unlock()
		} else {
			m.config.Resume()
			m.fail(err)
		}
		return
	}
	if err := m.phase(PhaseRestarting, "程序已替换，等待服务重启确认"); err != nil {
		m.restoreAfterSwitchFailure(err)
		return
	}
	if err := m.config.RequestRestart(); err != nil {
		m.restoreAfterSwitchFailure(err)
	}
}

func (m *Manager) Rollback(ctx context.Context, reason string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if strings.TrimSpace(reason) == "" {
		return m.snapshotLocked(), errors.New("请填写回退原因")
	}
	target := m.state.RollbackVersion
	if target == "" {
		return m.snapshotLocked(), errors.New("没有可回退的程序备份")
	}
	if m.state.Operation.Phase.Active() && m.state.Operation.Kind == "rollback" {
		return m.snapshotLocked(), nil
	}
	if err := m.beginLocked("rollback", target); err != nil {
		return m.snapshotLocked(), err
	}
	if m.running {
		return m.snapshotLocked(), nil
	}
	m.running = true
	m.operations.Add(1)
	go func() {
		defer m.operations.Done()
		defer func() { m.mu.Lock(); m.running = false; m.mu.Unlock() }()
		ctx, cancel := context.WithTimeout(m.lifetime, 15*time.Minute)
		defer cancel()
		if err := m.phase(PhaseValidating, "检查备份程序与当前数据库兼容性；不恢复数据库"); err != nil {
			m.fail(err)
			return
		}
		info, err := m.checkBinary(ctx, m.config.ManagedBinaryPath+".backup", "--check-schema")
		if err != nil {
			m.fail(err)
			return
		}
		if releaseTag(info.Version) != releaseTag(target) {
			m.fail(errors.New("备份程序版本与记录不一致"))
			return
		}
		candidate, err := copyCandidate(m.config.ManagedBinaryPath + ".backup")
		if err != nil {
			m.fail(err)
			return
		}
		defer os.Remove(candidate)
		m.activate(ctx, candidate)
	}()
	return m.snapshotLocked(), nil
}

func (m *Manager) ConfirmStarted() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.supported {
		return nil
	}
	op := &m.state.Operation
	if op.Phase != PhaseSwitching && op.Phase != PhaseRestarting {
		return nil
	}
	if CompareVersions(m.config.CurrentVersion, op.TargetVersion) != 0 {
		m.state.RollbackVersion = m.state.PreviousRollbackVersion
		m.finishLocked(PhaseFailed, "重启后仍在运行原版本，目标程序未生效")
		return m.saveLocked()
	}
	phase := PhaseSucceeded
	message := ""
	if op.Kind == "rollback" || op.AutomaticRollback {
		phase = PhaseRolledBack
	}
	if op.AutomaticRollback {
		message = op.Error
	}
	m.finishLocked(phase, message)
	m.logLocked("程序已启动并确认版本 " + m.config.CurrentVersion)
	return m.saveLocked()
}

// FailStartup only restores a pending candidate once, and never restores data.
func (m *Manager) FailStartup(cause error) error {
	m.mu.Lock()
	if !m.supported {
		m.mu.Unlock()
		return nil
	}
	op := m.state.Operation
	pending := op.Phase == PhaseSwitching || op.Phase == PhaseRestarting
	if !pending || CompareVersions(m.config.CurrentVersion, op.TargetVersion) != 0 {
		m.mu.Unlock()
		return nil
	}
	if op.AutomaticRollback {
		m.finishLocked(PhaseManualIntervention, "程序启动失败，需要人工恢复")
		_ = m.saveLocked()
		m.mu.Unlock()
		return nil
	}
	m.finishLocked(PhaseFailed, "新版程序启动失败，正在检查旧程序恢复条件")
	if err := m.saveLocked(); err != nil {
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	info, err := m.checkBinary(ctx, m.config.ManagedBinaryPath+".backup", "--check-schema")
	if err == nil && CompareVersions(info.Version, op.FromVersion) != 0 {
		err = errors.New("备份版本与升级前版本不一致")
	}
	if err == nil {
		var candidate string
		candidate, err = copyCandidate(m.config.ManagedBinaryPath + ".backup")
		if err == nil {
			defer os.Remove(candidate)
			err = swapProgram(m.config.ManagedBinaryPath, candidate)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.finishLocked(PhaseManualIntervention, "新版启动失败，旧程序无法安全恢复："+err.Error())
		return errors.Join(err, m.saveLocked())
	}
	m.state.RollbackVersion = op.TargetVersion
	m.state.Operation.AutomaticRollback = true
	m.state.Operation.Kind = "rollback"
	m.state.Operation.FromVersion = op.TargetVersion
	m.state.Operation.TargetVersion = op.FromVersion
	m.state.Operation.Phase = PhaseRestarting
	m.state.Operation.FinishedAt = nil
	m.state.Operation.Error = "新版程序启动失败，已恢复兼容的旧程序；数据库未恢复"
	m.logLocked(m.state.Operation.Error)
	return m.saveLocked()
}

func (m *Manager) restoreAfterSwitchFailure(cause error) {
	candidate, err := copyCandidate(m.config.ManagedBinaryPath + ".backup")
	if err == nil {
		defer os.Remove(candidate)
		err = swapProgram(m.config.ManagedBinaryPath, candidate)
	}
	if err == nil {
		m.config.Resume()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.finishLocked(PhaseManualIntervention, "重启请求失败且程序恢复失败，请人工处理")
	} else {
		m.state.RollbackVersion = m.state.Operation.TargetVersion
		m.finishLocked(PhaseFailed, "重启请求失败，原程序已恢复："+cause.Error())
	}
	_ = m.saveLocked()
}

func (m *Manager) phase(phase Phase, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Operation.Phase = phase
	m.logLocked(message)
	return m.saveLocked()
}

func (m *Manager) fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.finishLocked(PhaseFailed, err.Error())
	_ = m.saveLocked()
}

func (m *Manager) finishLocked(phase Phase, message string) {
	now := time.Now().UTC()
	m.state.Operation.Phase = phase
	m.state.Operation.FinishedAt = &now
	m.state.Operation.Error = message
	if message != "" {
		m.logLocked(message)
	}
}

func (m *Manager) logLocked(message string) {
	m.state.Operation.Logs = append(m.state.Operation.Logs, LogEntry{At: time.Now().UTC(), Phase: m.state.Operation.Phase, Message: message})
	if len(m.state.Operation.Logs) > 100 {
		m.state.Operation.Logs = m.state.Operation.Logs[len(m.state.Operation.Logs)-100:]
	}
}

func (m *Manager) saveLocked() error { return writeState(m.statePath(), m.state) }

func writeState(path string, state persistedState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".update-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

type unsafeSwapError struct{ err error }

func (e *unsafeSwapError) Error() string { return e.err.Error() }
func (e *unsafeSwapError) Unwrap() error { return e.err }

func swapProgram(path, candidate string) error {
	return swapProgramWithRename(path, candidate, os.Rename)
}

func swapProgramWithRename(path, candidate string, rename func(string, string) error) error {
	if filepath.Dir(path) != filepath.Dir(candidate) {
		return errors.New("新旧程序必须位于同一文件系统目录")
	}
	if info, err := os.Lstat(candidate); err != nil || !info.Mode().IsRegular() {
		return errors.New("候选程序不存在或不是普通文件")
	}
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		return errors.New("当前程序不存在或不是普通文件")
	}
	if err := rename(path, path+".backup"); err != nil {
		return fmt.Errorf("保留旧程序失败：%w", err)
	}
	if err := rename(candidate, path); err != nil {
		if restoreErr := rename(path+".backup", path); restoreErr != nil {
			return &unsafeSwapError{fmt.Errorf("替换失败且恢复旧程序失败：%w", restoreErr)}
		}
		return fmt.Errorf("替换程序失败，旧程序已恢复：%w", err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		if restoreErr := rename(path+".backup", path); restoreErr != nil {
			return &unsafeSwapError{fmt.Errorf("程序交换落盘失败且原程序恢复失败：%w", restoreErr)}
		}
		return fmt.Errorf("程序交换落盘失败，原程序已恢复：%w", err)
	}
	return nil
}

func copyCandidate(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("程序备份不存在或不是普通文件")
	}
	source, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer source.Close()
	target, err := os.CreateTemp(filepath.Dir(path), ".yingce-backend-new-*")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		target.Close()
		if !ok {
			os.Remove(target.Name())
		}
	}()
	size, err := io.Copy(target, io.LimitReader(source, maxBinaryBytes+1))
	if err != nil || size != info.Size() || size > maxBinaryBytes {
		return "", errors.New("程序备份复制失败或超过大小限制")
	}
	if err := target.Chmod(0o700); err != nil {
		return "", err
	}
	if err := target.Sync(); err != nil {
		return "", err
	}
	if err := target.Close(); err != nil {
		return "", err
	}
	ok = true
	return target.Name(), nil
}

type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 64<<10 {
		return 0, errors.New("程序自检输出过大")
	}
	return b.Buffer.Write(p)
}

func runBinaryCheck(ctx context.Context, path, mode string) (buildinfo.Info, error) {
	var info buildinfo.Info
	stat, err := os.Lstat(path)
	if err != nil || !stat.Mode().IsRegular() {
		return info, errors.New("自检程序不存在或不是普通文件")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, mode)
	cleanup := prepareCheckCommand(command)
	defer cleanup()
	var output boundedOutput
	command.Stdout = &output
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		if mode == "--check" {
			return info, errors.New("候选程序自检失败，请先检查 Node.js 与系统库兼容性，必要时更新官方 Docker 运行环境镜像")
		}
		return info, errors.New("备份程序自检失败，请检查当前数据库结构与备份程序的兼容性；未恢复数据库")
	}
	if json.Unmarshal(output.Bytes(), &info) != nil || !IsReleaseVersion(info.Version) {
		return info, errors.New("程序自检未返回有效版本信息")
	}
	return info, nil
}

// RestoreProgram is the offline repair path; it checks the current schema without changing it.
func RestoreProgram(ctx context.Context, managedPath, dataDir string) error {
	if managedPath == "" || !filepath.IsAbs(managedPath) {
		return errors.New("必须明确指定受管程序绝对路径")
	}
	statePath := updateStatePath(dataDir)
	stateDir := filepath.Dir(statePath)
	lock, err := lockUpdate(stateDir)
	if err != nil {
		return err
	}
	defer lock.Close()
	info, err := runBinaryCheck(ctx, managedPath+".backup", "--check-schema")
	if err != nil {
		return err
	}
	candidate, err := copyCandidate(managedPath + ".backup")
	if err != nil {
		return err
	}
	defer os.Remove(candidate)
	state := persistedState{FormatVersion: 1, Operation: Operation{Phase: PhaseRestarting, Kind: "rollback", TargetVersion: info.Version, Logs: []LogEntry{}}}
	if data, err := os.ReadFile(statePath); err == nil {
		if len(data) > 1<<20 || json.Unmarshal(data, &state) != nil || state.FormatVersion != 1 {
			return errors.New("更新状态损坏，请人工检查后恢复")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	state.Operation.FromVersion = state.Operation.TargetVersion
	state.Operation.Kind = "rollback"
	state.Operation.Phase = PhaseRestarting
	state.Operation.TargetVersion = info.Version
	state.Operation.FinishedAt = nil
	state.Operation.AutomaticRollback = false
	state.Operation.Error = ""
	now := time.Now().UTC()
	state.Operation.StartedAt = &now
	state.Operation.Logs = append(state.Operation.Logs, LogEntry{At: now, Phase: PhaseRestarting, Message: "已通过旧程序数据库只读检查，执行人工程序恢复"})
	current, err := runBinaryCheck(ctx, managedPath, "--check")
	if err == nil {
		state.RollbackVersion = current.Version
		state.Operation.FromVersion = current.Version
	} else {
		state.RollbackVersion = ""
	}
	if err := writeState(statePath, state); err != nil {
		return err
	}
	if _, err := os.Lstat(managedPath); errors.Is(err, os.ErrNotExist) {
		err = os.Rename(candidate, managedPath)
		if err == nil {
			err = syncDirectory(filepath.Dir(managedPath))
		}
		if err == nil {
			return nil
		}
		state.Operation.Phase = PhaseFailed
		state.Operation.Error = "恢复程序失败：" + err.Error()
		state.Operation.FinishedAt = &now
		return errors.Join(err, writeState(statePath, state))
	}
	if err := swapProgram(managedPath, candidate); err != nil {
		state.Operation.Phase = PhaseFailed
		state.Operation.Error = "恢复程序失败：" + err.Error()
		state.Operation.FinishedAt = &now
		return errors.Join(err, writeState(statePath, state))
	}
	return nil
}
