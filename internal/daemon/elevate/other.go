//go:build !linux

package elevate

import "log"

func Needed(error) bool { return false }

func Tun(*log.Logger, string) {}

func Restarted() bool { return false }
