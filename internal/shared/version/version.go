package version

import "strings"

var Version = "1.3.2"

func String() string {
	return "v" + strings.TrimPrefix(Version, "v")
}
