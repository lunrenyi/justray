//go:build !linux

package daemon

import "log"

func tunPermissionErr(err error) bool { return false }

func elevateTun(*log.Logger, string) {}
