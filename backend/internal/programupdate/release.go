package programupdate

import (
	"archive/tar"
	"bufio"
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
	"path"
	"strings"
	"time"
)

const (
	maxReleaseBytes  = 4 << 20
	maxArchiveBytes  = 512 << 20
	maxBinaryBytes   = 1 << 30
	maxChecksumBytes = 1 << 20
	checkCacheTTL    = 20 * time.Minute
)

var ErrNoInstallableRelease = errors.New("尚无可安装的程序 Release")

func safeURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "api.github.com", "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return true
	}
	return false
}

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !safeURL(req.URL.String()) {
			return errors.New("更新下载重定向地址不受信任")
		}
		return nil
	}}
}

func (m *Manager) get(ctx context.Context, rawURL string) (*http.Response, error) {
	if !safeURL(rawURL) {
		return nil, errors.New("更新下载地址不受信任")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, errors.New("无法创建更新下载请求")
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "open-ai-canvas-program-update")
	response, err := m.client.Do(req)
	if err != nil {
		return nil, errors.New("无法连接 GitHub 更新服务")
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("GitHub 更新服务返回 HTTP %d", response.StatusCode)
	}
	return response, nil
}

func (m *Manager) fetchReleases(ctx context.Context) ([]Release, error) {
	response, err := m.get(ctx, "https://api.github.com/repos/"+Repository+"/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxReleaseBytes+1))
	if err != nil || len(data) > maxReleaseBytes {
		return nil, errors.New("版本列表过大或读取失败")
	}
	var releases []struct {
		Tag         string    `json:"tag_name"`
		Name        string    `json:"name"`
		Body        string    `json:"body"`
		URL         string    `json:"html_url"`
		PublishedAt time.Time `json:"published_at"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		Assets      []asset   `json:"assets"`
	}
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, errors.New("GitHub 版本列表格式无效")
	}
	result := make([]Release, 0, len(releases))
	for _, release := range releases {
		if release.Draft || release.Prerelease || !IsReleaseVersion(release.Tag) {
			continue
		}
		result = append(result, Release{Version: releaseTag(release.Tag), Name: release.Name, Body: release.Body, URL: release.URL, PublishedAt: release.PublishedAt, Assets: release.Assets})
	}
	return result, nil
}

func (m *Manager) releaseFor(ctx context.Context, target string) (*Release, error) {
	releases, err := m.fetchReleases(ctx)
	if err != nil {
		return nil, err
	}
	for _, release := range releases {
		if release.Version == releaseTag(target) {
			if _, _, err := m.installableAssets(&release); err != nil {
				return nil, fmt.Errorf("%w（%s）：%s", ErrNoInstallableRelease, release.Version, err)
			}
			return &release, nil
		}
	}
	return nil, errors.New("未找到指定的正式 Release")
}

func assetNamed(release *Release, name string) (asset, error) {
	var found *asset
	for i := range release.Assets {
		if release.Assets[i].Name == name {
			if found != nil {
				return asset{}, errors.New("Release 包含重复的更新资产")
			}
			found = &release.Assets[i]
		}
	}
	if found == nil || found.URL != "https://github.com/"+Repository+"/releases/download/"+release.Version+"/"+name || !safeURL(found.URL) {
		return asset{}, fmt.Errorf("Release 缺少有效的 %s", name)
	}
	return *found, nil
}

func (m *Manager) installableAssets(release *Release) (asset, asset, error) {
	name := fmt.Sprintf("open-ai-canvas_%s_linux_%s.tar.gz", release.Version, m.arch)
	archive, err := assetNamed(release, name)
	if err != nil {
		return asset{}, asset{}, err
	}
	if archive.Size <= 0 || archive.Size > maxArchiveBytes {
		return asset{}, asset{}, errors.New("更新包大小不符合限制")
	}
	sums, err := assetNamed(release, "SHA256SUMS")
	if err != nil {
		return asset{}, asset{}, err
	}
	if sums.Size <= 0 || sums.Size > maxChecksumBytes {
		return asset{}, asset{}, errors.New("校验文件大小不符合限制")
	}
	return archive, sums, nil
}

func (m *Manager) downloadCandidate(ctx context.Context, release *Release) (string, error) {
	archive, sums, err := m.installableAssets(release)
	if err != nil {
		return "", err
	}
	response, err := m.get(ctx, sums.URL)
	if err != nil {
		return "", err
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, maxChecksumBytes+1))
	response.Body.Close()
	if readErr != nil || len(data) > maxChecksumBytes {
		return "", errors.New("无法读取更新包校验文件")
	}
	expected, err := checksumFor(data, archive.Name)
	if err != nil {
		return "", err
	}
	response, err = m.get(ctx, archive.URL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	file, err := os.CreateTemp(m.binDir(), ".download-*")
	if err != nil {
		return "", fmt.Errorf("创建更新临时文件：%w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, maxArchiveBytes+1))
	if err != nil || size != archive.Size || size > maxArchiveBytes {
		return "", errors.New("更新包下载不完整或超过大小限制")
	}
	if hex.EncodeToString(hash.Sum(nil)) != expected {
		return "", errors.New("更新包 SHA256 校验失败")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	candidate, err := os.CreateTemp(m.binDir(), ".yingce-backend-new-*")
	if err != nil {
		return "", err
	}
	nameOnDisk := candidate.Name()
	ok := false
	defer func() {
		candidate.Close()
		if !ok {
			os.Remove(nameOnDisk)
		}
	}()
	if err := extractBinary(file, candidate); err != nil {
		return "", err
	}
	if err := candidate.Chmod(0o700); err != nil {
		return "", err
	}
	if err := candidate.Sync(); err != nil {
		return "", err
	}
	if err := candidate.Close(); err != nil {
		return "", err
	}
	ok = true
	return nameOnDisk, nil
}

func checksumFor(data []byte, name string) (string, error) {
	var expected string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		if expected != "" {
			return "", errors.New("更新包校验记录重复")
		}
		decoded, err := hex.DecodeString(fields[0])
		if err != nil || len(decoded) != sha256.Size {
			return "", errors.New("更新包校验值无效")
		}
		expected = strings.ToLower(fields[0])
	}
	if scanner.Err() != nil || expected == "" {
		return "", errors.New("校验文件缺少更新包记录")
	}
	return expected, nil
}

func extractBinary(source io.Reader, target io.Writer) error {
	gz, err := gzip.NewReader(source)
	if err != nil {
		return errors.New("更新包不是有效 gzip 文件")
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	var total int64
	found := false
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.New("更新归档格式无效")
		}
		name := strings.TrimPrefix(header.Name, "./")
		if path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || strings.Contains(name, "\\") {
			return errors.New("更新归档包含不安全路径")
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return errors.New("更新归档只能包含普通文件")
		}
		if header.Size < 0 || header.Size > maxBinaryBytes || total > maxBinaryBytes-header.Size {
			return errors.New("更新归档解压大小超过限制")
		}
		total += header.Size
		if name == "yingce-backend" {
			if found || header.Size == 0 {
				return errors.New("更新归档中的程序文件无效或重复")
			}
			found = true
			if _, err := io.Copy(target, reader); err != nil {
				return errors.New("解压更新程序失败")
			}
		}
	}
	if !found {
		return errors.New("更新归档缺少 yingce-backend")
	}
	// Reading through gzip EOF validates its trailer as well as the tar entries.
	if size, err := io.Copy(io.Discard, io.LimitReader(gz, maxBinaryBytes-total+1)); err != nil || size > maxBinaryBytes-total {
		return errors.New("更新归档校验失败")
	}
	return nil
}
