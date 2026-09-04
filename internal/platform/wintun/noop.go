//go:build !windows

package wintun

func Ensure() (string, error) { return "", nil }
