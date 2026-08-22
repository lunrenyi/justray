//go:build windows

package wintun

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed wintun_amd64.dll wintun_arm64.dll
var blobs embed.FS

var names = map[string]string{
	"amd64": "wintun_amd64.dll",
	"arm64": "wintun_arm64.dll",
}

func Ensure() (string, error) {
	name, ok := names[runtime.GOARCH]
	if !ok {
		return "", nil
	}
	blob, err := blobs.ReadFile(name)
	if err != nil {
		return "", err
	}

	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dst := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if info, err := os.Stat(dst); err == nil && info.Size() == int64(len(blob)) {
		return dst, nil
	}
	if err := os.WriteFile(dst, blob, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", dst, err)
	}
	return dst, nil
}
