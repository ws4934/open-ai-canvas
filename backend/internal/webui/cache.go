package webui

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

type assetCache struct {
	directory string
	versions  []string
}

type cacheState struct {
	Current   string               `json:"current"`
	Previous  string               `json:"previous,omitempty"`
	Published map[string]time.Time `json:"published"`
}

func publishAssets(files fs.FS, dataDir, identity string, now time.Time) (*assetCache, error) {
	if identity == "" || filepath.Base(identity) != identity || identity == "." || identity == ".." {
		return nil, errors.New("invalid frontend build identity")
	}
	directory := filepath.Join(dataDir, "runtime", "web-assets")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(directory, ".lock"))
	if err := lock.Lock(); err != nil {
		return nil, err
	}
	defer lock.Unlock()
	state := cacheState{Published: make(map[string]time.Time)}
	statePath := filepath.Join(directory, "state.json")
	data, err := os.ReadFile(statePath)
	if err == nil {
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if state.Published == nil {
		state.Published = make(map[string]time.Time)
	}
	if _, err := os.Stat(filepath.Join(directory, identity)); errors.Is(err, os.ErrNotExist) {
		stage, err := os.MkdirTemp(directory, ".stage-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(stage)
		err = fs.WalkDir(files, "assets", func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			target := filepath.Join(stage, filepath.FromSlash(name))
			if entry.IsDir() {
				return os.MkdirAll(target, 0o700)
			}
			if !entry.Type().IsRegular() {
				return errors.New("frontend assets must be regular files")
			}
			source, err := files.Open(name)
			if err != nil {
				return err
			}
			defer source.Close()
			destination, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(destination, source)
			return errors.Join(copyErr, destination.Close())
		})
		if err != nil {
			return nil, err
		}
		if err := os.Rename(stage, filepath.Join(directory, identity)); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if state.Current != identity {
		state.Previous = state.Current
		state.Current = identity
	}
	state.Published[identity] = now
	cache := &assetCache{directory: directory}
	for version, published := range state.Published {
		if version == "" || filepath.Base(version) != version || version == "." || version == ".." {
			return nil, errors.New("invalid cached build identity")
		}
		if version != state.Current && version != state.Previous && now.Sub(published) > 7*24*time.Hour {
			if err := os.RemoveAll(filepath.Join(directory, version)); err != nil {
				return nil, err
			}
			delete(state.Published, version)
		} else {
			cache.versions = append(cache.versions, version)
		}
	}
	data, err = json.Marshal(state)
	if err != nil {
		return nil, err
	}
	stage, err := os.CreateTemp(directory, ".state-")
	if err != nil {
		return nil, err
	}
	defer os.Remove(stage.Name())
	_, writeErr := stage.Write(data)
	if err := errors.Join(writeErr, stage.Close()); err != nil {
		return nil, err
	}
	if err := os.Rename(stage.Name(), statePath); err != nil {
		return nil, err
	}
	return cache, nil
}
