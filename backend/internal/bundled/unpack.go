package bundled

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

const maxUnpackedBytes = 2 << 30
const maxArchiveEntries = 100000

func unpack(source io.Reader, directory string) error {
	compressed, err := gzip.NewReader(source)
	if err != nil {
		return fmt.Errorf("open bundled runtime archive: %w", err)
	}
	defer compressed.Close()
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	reader := tar.NewReader(compressed)
	var total int64
	for count := 0; ; count++ {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read bundled runtime archive: %w", err)
		}
		if count >= maxArchiveEntries {
			return errors.New("bundled runtime archive contains too many entries")
		}
		name := strings.TrimSuffix(strings.TrimPrefix(header.Name, "./"), "/")
		if strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || path.Clean(name) != name || name == "." || name == ".." || strings.HasPrefix(name, "../") {
			return fmt.Errorf("invalid bundled runtime path %q", header.Name)
		}
		allowed := name == "agent-runtime" || name == "agent-runtime/pi" || strings.HasPrefix(name, "agent-runtime/pi/") || name == "plugin-packages" || strings.HasPrefix(name, "plugin-packages/")
		if !allowed {
			return fmt.Errorf("unexpected bundled runtime path %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o700); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxUnpackedBytes-total {
				return errors.New("bundled runtime archive exceeds size limit")
			}
			total += header.Size
			if err := root.MkdirAll(path.Dir(name), 0o700); err != nil {
				return err
			}
			file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600|os.FileMode(header.Mode)&0o111)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			if err := errors.Join(copyErr, file.Close()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("bundled runtime entry %q is not a regular file or directory", header.Name)
		}
	}
	// Consume the gzip trailer so truncated or corrupted archives cannot be used.
	if count, err := io.Copy(io.Discard, io.LimitReader(compressed, 1<<20)); err != nil {
		return err
	} else if count == 1<<20 {
		return errors.New("bundled runtime archive has excessive trailing data")
	}
	return nil
}
