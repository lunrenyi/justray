package main

//
// CLIENT ENTRYPOINT
//

import (
	"fmt"
	"os"

	"github.com/luynrs/justray/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "justray:", err)
		os.Exit(1)
	}
}
