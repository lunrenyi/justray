package cli

import (
	"debug/buildinfo"
	"runtime"

	"github.com/luynrs/justray/internal/client/tui/style"
	"github.com/luynrs/justray/internal/version"
)

func versionBlock() string {
	var pairs [][2]string
	if v := singboxVersion(); v != "" {
		pairs = append(pairs, [2]string{"sing-box", v})
	}
	pairs = append(pairs, [2]string{"Platform", runtime.GOOS + "/" + runtime.GOARCH})

	head := style.Dim.Render("·") + " JustRay " + style.Dim.Render(version.String())
	return head + "\n" + fieldLines(pairs...) + "\n"
}

func singboxVersion() string {
	bin, err := justrayd()
	if err != nil {
		return ""
	}
	info, err := buildinfo.ReadFile(bin)
	if err != nil {
		return ""
	}
	for _, d := range info.Deps {
		if d.Path == "github.com/sagernet/sing-box" {
			return d.Version
		}
	}
	return ""
}
