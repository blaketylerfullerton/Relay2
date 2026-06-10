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

// New builds an App and chooses the cluster backend from the environment:
//
//	RELAY_DEMO=1          → canned multi-node mock (screenshots, UI without runtimes)
//	RELAY_CONTROLLER=addr → live cluster read from the controller at addr
//	(neither)             → this machine only, via local runtime discovery
//
// The Store interface is the seam: every command, the scheduler, and the
// renderer are identical across all three.
func New() *App {
	var s store.Store
	switch {
	case os.Getenv("RELAY_DEMO") == "1":
		s = store.NewMock()
	case os.Getenv("RELAY_CONTROLLER") != "":
		s = store.NewController(os.Getenv("RELAY_CONTROLLER"))
	default:
		s = store.NewLocal()
	}
	return &App{Store: s, Out: os.Stdout, Err: os.Stderr}
}

const usage = `relay — coordinate an entire AI infrastructure

Usage:
  relay <command> [args]

Commands:
  controller        Run the cluster controller agents register with
  join              Join this machine to the cluster (--controller addr)
  nodes             List machines in the cluster
  models            List models available to run
  run <model>       Schedule a model onto the best node (--explain to see why)
  status            One-glance cluster health and capacity
  watch             Live dashboard of the whole fabric
  update            Update this binary (rebuild from source, or download a release)
  version           Print the installed version

Point the read commands at a controller with RELAY_CONTROLLER=host:port.

Run 'relay <command> --help' for details.`

// Run dispatches a single invocation. args excludes the program name.
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.Out, usage)
		return 0
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "controller":
		return a.cmdController(rest)
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
	case "version", "--version", "-v":
		fmt.Fprintln(a.Out, Version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprintln(a.Out, usage)
		return 0
	default:
		fmt.Fprintf(a.Err, "relay: unknown command %q\n\n%s\n", cmd, usage)
		return 2
	}
}
