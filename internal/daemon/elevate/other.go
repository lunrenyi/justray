//go:build !linux

package elevate

import "github.com/charmbracelet/log"

func Needed(error) bool { return false }

func Tun(*log.Logger, string) {}
