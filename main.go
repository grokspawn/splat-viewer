package main

import (
	"os"

	"github.com/operator-framework/splat-viewer/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
