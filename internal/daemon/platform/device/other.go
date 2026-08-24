//go:build !windows

package device

func Info() (hwid, ver, model string) { return "", "", "" }
