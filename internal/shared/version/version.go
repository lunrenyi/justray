package version

import "strings"

var Version = "1.1.0"

func String() string { return "v" + strings.TrimPrefix(Version, "v") }
