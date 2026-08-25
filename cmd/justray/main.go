package main

// CLIENT ENTRYPOINT

import (
	"os"

	"github.com/luynrs/justray/internal/client/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		cli.Fail(err)
		os.Exit(1)
	}
}
