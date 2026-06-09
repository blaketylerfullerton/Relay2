// Command relay is the CLI for the Relay compute fabric.
//
// Phase 0 is the scheduler and inventory layer: it does not run inference
// itself, it orchestrates the agents and inference servers that do.
package main

import (
	"os"

	"relay/internal/cli"
)

func main() {
	os.Exit(cli.New().Run(os.Args[1:]))
}
