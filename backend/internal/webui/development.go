//go:build !embedweb

package webui

import "io/fs"

func releaseFiles() (fs.FS, error) { return nil, nil }
