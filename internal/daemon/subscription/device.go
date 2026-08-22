package subscription

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/luynrs/justray/internal/daemon/platform/console"
)

// Device info; X-Hwid is required, the rest cosmetic
func deviceHeaders() (http.Header, error) {
	h := http.Header{}
	set := func(key, val string) {
		if val != "" {
			h.Set(key, val)
		}
	}
	set("User-Agent", "justray")

	switch runtime.GOOS {
	case "linux":
		set("X-Device-OS", "Linux")
		set("X-Hwid", hash(cmp.Or(readFile("/etc/machine-id"), readFile("/var/lib/dbus/machine-id"))))
		set("X-Ver-OS", distro())
		set("X-Device-Model", readFile("/sys/devices/virtual/dmi/id/product_name"))
	case "darwin":
		_, uuid, _ := strings.Cut(run("ioreg", "-rd1", "-c", "IOPlatformExpertDevice"), `"IOPlatformUUID" = "`)
		uuid, _, _ = strings.Cut(uuid, `"`)
		set("X-Device-OS", "macOS")
		set("X-Hwid", hash(uuid))
		set("X-Ver-OS", run("sw_vers", "-productVersion"))
		set("X-Device-Model", run("sysctl", "-n", "hw.model"))
	case "windows":
		ps := func(q string) string { // wmic is gone as of Win11 24H2
			return run("powershell", "-NoProfile", "-Command", q)
		}
		set("X-Device-OS", "Windows")
		set("X-Hwid", hash(ps("(Get-CimInstance Win32_ComputerSystemProduct).UUID")))
		set("X-Ver-OS", ps("(Get-CimInstance Win32_OperatingSystem).Version"))
		set("X-Device-Model", ps("(Get-CimInstance Win32_ComputerSystem).Model"))
	default:
		set("X-Device-OS", runtime.GOOS)
	}

	if h.Get("X-Hwid") == "" {
		return h, fmt.Errorf("no machine id on %s", runtime.GOOS)
	}
	return h, nil
}

func hash(raw string) string {
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("justray:" + raw))
	return hex.EncodeToString(sum[:16]) // 32 hex chars to match [a-zA-Z0-9=-]{10,64}$
}

func distro() string {
	for line := range strings.SplitSeq(readFile("/etc/os-release"), "\n") {
		if name, ok := strings.CutPrefix(line, "PRETTY_NAME="); ok {
			return strings.Trim(name, `"`)
		}
	}
	return ""
}

func readFile(p string) string {
	data, _ := os.ReadFile(p)
	return strings.TrimSpace(string(data))
}

func run(name string, arg ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, arg...)
	console.Hide(cmd)
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}
