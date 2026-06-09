// Package cli wires the `relay` subcommands to the store and renderer.
//
// Dispatch is a plain switch rather than a framework: five commands don't
// justify a dependency, and the seam that matters (the Store) lives elsewhere.
package cli

import (
	"fmt"
	"io"
	"os"

	"relay/internal/store"
)

// App holds the shared dependencies a command needs.
type App struct {
	Store store.Store
	Out   io.Writer
	Err   io.Writer
}

// New builds an App backed by the real local store and stdio. Set RELAY_DEMO=1
// to use the canned multi-node mock cluster instead (useful for screenshots
// and trying the UI without live runtimes).
func New() *App {
	var s store.Store = store.NewLocal()
	if os.Getenv("RELAY_DEMO") == "1" {
		s = store.NewMock()
	}
	return &App{Store: s, Out: os.Stdout, Err: os.Stderr}
}

const usage = `relay — coordinate an entire AI infrastructure

Usage:
  relay <command> [args]

Commands:
  join              Install the agent and join this machine to the cluster
  nodes             List machines in the cluster
  models            List models available to run
  run <model>       Schedule a model onto the best node (--explain to see why)
  status            One-glance cluster health and capacity
  watch             Live dashboard of the whole fabric
  update            Pull latest source, rebuild, and replace this binary

Run 'relay <command> --help' for details.`

// Run dispatches a single invocation. args excludes the program name.
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.Out, usage)
		return 0
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "join":
		return a.cmdJoin(rest)
	case "nodes":
		return a.cmdNodes(rest)
	case "models":
		return a.cmdModels(rest)
	case "status":
		return a.cmdStatus(rest)
	case "run":
		return a.cmdRun(rest)
	case "watch":
		return a.cmdWatch(rest)
	case "update":
		return a.cmdUpdate(rest)
	case "help", "-h", "--help":
		fmt.Fprintln(a.Out, usage)
		return 0
	default:
		fmt.Fprintf(a.Err, "relay: unknown command %q\n\n%s\n", cmd, usage)
		return 2
	}
}
