package version

import "strings"

var Version = "1.4.4"

func String() string {
	return "v" + strings.TrimPrefix(Version, "v")
}
