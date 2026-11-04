package programupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func installableRelease(version, arch string) Release {
	name := fmt.Sprintf("open-ai-canvas_%s_linux_%s.tar.gz", version, arch)
	base := "https://github.com/" + Repository + "/releases/download/" + version + "/"
	return Release{Version: version, Assets: []asset{
		{Name: name, URL: base + name, Size: 1024},
		{Name: "SHA256SUMS", URL: base + "SHA256SUMS", Size: 256},
	}}
}

func releaseList(releases ...Release) []byte {
	items := make([]map[string]any, 0, len(releases))
	for _, release := range releases {
		items = append(items, map[string]any{"tag_name": release.Version, "assets": release.Assets})
	}
	data, _ := json.Marshal(items)
	return data
}

func TestProgramUpdateRejectsUninstallableReleaseAssets(t *testing.T) {
	for _, scenario := range []string{"missing-program", "missing-checksum", "other-architecture", "untrusted-url", "other-repository", "wrong-file-url", "zero-archive", "oversized-archive", "zero-checksum", "oversized-checksum"} {
		t.Run(scenario, func(t *testing.T) {
			m := testManager(t)
			release := installableRelease("v1.0.0.9", "amd64")
			switch scenario {
			case "missing-program":
				release.Assets = []asset{release.Assets[1]}
			case "missing-checksum":
				release.Assets = release.Assets[:1]
			case "other-architecture":
				release = installableRelease(release.Version, "arm64")
			case "untrusted-url":
				release.Assets[0].URL = "https://example.com/program.tar.gz"
			case "other-repository":
				release.Assets[0].URL = strings.Replace(release.Assets[0].URL, "/"+Repository+"/", "/another-owner/open-ai-canvas/", 1)
			case "wrong-file-url":
				release.Assets[0].URL = release.Assets[1].URL
			case "zero-archive":
				release.Assets[0].Size = 0
			case "oversized-archive":
				release.Assets[0].Size = maxArchiveBytes + 1
			case "zero-checksum":
				release.Assets[1].Size = 0
			case "oversized-checksum":
				release.Assets[1].Size = maxChecksumBytes + 1
			}
			data := releaseList(release)
			m.client = &http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
			})}
			status, err := m.Check(context.Background(), false)
			if !errors.Is(err, ErrNoInstallableRelease) || status.LatestRelease != nil || status.UpdateAvailable {
				t.Fatalf("recommended unusable release: %+v err=%v", status, err)
			}
			if _, err := m.releaseFor(context.Background(), release.Version); !errors.Is(err, ErrNoInstallableRelease) {
				t.Fatal("start did not revalidate release assets", err)
			}
		})
	}
}

func TestProgramUpdateInstallableReleaseCacheAndForce(t *testing.T) {
	m := testManager(t)
	valid := installableRelease("v1.0.0.2", "amd64")
	incomplete := installableRelease("v1.0.0.9", "amd64")
	incomplete.Assets = incomplete.Assets[1:]
	data := releaseList(incomplete, valid)
	var calls atomic.Int32
	m.client = &http.Client{Transport: testTransport(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
	})}
	status, err := m.Check(context.Background(), false)
	if err != nil || status.LatestRelease.Version != valid.Version {
		t.Fatalf("did not skip higher release without a program archive: %+v %v", status, err)
	}
	data = releaseList(incomplete)
	status, err = m.Check(context.Background(), false)
	if err != nil || status.LatestRelease.Version != valid.Version || calls.Load() != 1 {
		t.Fatal("automatic check did not reuse installable cache", status, err)
	}
	status, err = m.Check(context.Background(), true)
	if !errors.Is(err, ErrNoInstallableRelease) || status.LatestRelease != nil || status.UpdateAvailable || calls.Load() != 2 {
		t.Fatal("force refresh retained unavailable candidate", status, err)
	}
	data = releaseList(valid)
	status, err = m.Check(context.Background(), false)
	if !errors.Is(err, ErrNoInstallableRelease) || status.LatestRelease != nil || calls.Load() != 2 {
		t.Fatal("negative cache lost its explicit error", status, err)
	}
	status, err = m.Check(context.Background(), true)
	if err != nil || status.LatestRelease.Version != valid.Version || calls.Load() != 3 {
		t.Fatal("force refresh did not discover newly uploaded artifacts", status, err)
	}
}

func TestProgramUpdateUsesForkRepository(t *testing.T) {
	if Repository != "ws4934/open-ai-canvas" {
		t.Fatalf("unexpected update repository: %s", Repository)
	}
	m := testManager(t)
	data := releaseList(installableRelease("v1.0.0.2", "amd64"))
	m.client = &http.Client{Transport: testTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://api.github.com/repos/ws4934/open-ai-canvas/releases?per_page=100" {
			t.Fatalf("release check queried another repository: %s", request.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
	})}
	status, err := m.Check(context.Background(), true)
	if err != nil || status.Repository != Repository || status.LatestRelease == nil {
		t.Fatalf("fork release check: %+v err=%v", status, err)
	}
}
