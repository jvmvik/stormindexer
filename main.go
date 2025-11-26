package main

import (
	"github.com/victor/stormindexer/cmd"
)

func main() {
	defer cmd.Cleanup()
	cmd.Execute()
}

