//go:build windows

package device

import (
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// hwid, OS version, model
func Info() (hwid, ver, model string) {
	reg := func(path, name string) (v string) {
		if k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE); err == nil {
			v, _, _ = k.GetStringValue(name)
			k.Close()
		}
		return
	}
	v := windows.RtlGetVersion()
	return reg(`SOFTWARE\Microsoft\Cryptography`, "MachineGuid"),
		fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber),
		reg(`HARDWARE\DESCRIPTION\System\BIOS`, "SystemProductName")
}
