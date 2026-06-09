package cli

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"relay/internal/agent"
	"relay/internal/render"
	rt "relay/internal/runtime"
	"relay/internal/scheduler"
	"relay/internal/types"
)

// cmdJoin inspects the local machine via the agent and reports what it would
// register with the controller. Phase 0 stops short of real registration
// (that's gRPC to the controller, a later phase), but the discovery is real.
func (a *App) cmdJoin(_ []string) int {
	fmt.Fprintln(a.Out, "Joining Relay cluster...")
	fmt.Fprintln(a.Out, "  inspecting host...")
	time.Sleep(120 * time.Millisecond)

	d := agent.Inspect()
	fmt.Fprint(a.Out, render.Discovery(d.Node, d.Runtimes, d.Models))

	if len(d.Runtimes) == 0 {
		fmt.Fprintln(a.Out, "\nNo inference runtime detected. Install Ollama, vLLM, or llama.cpp,")
		fmt.Fprintln(a.Out, "then re-run 'relay join'.")
		return 0
	}
	fmt.Fprintln(a.Out, "\nThis machine is ready to join. Try: relay nodes")
	return 0
}

// cmdNodes prints the compact inventory table.
func (a *App) cmdNodes(_ []string) int {
	c, err := a.Store.Snapshot()
	if err != nil {
		fmt.Fprintf(a.Err, "relay: %v\n", err)
		return 1
	}
	fmt.Fprint(a.Out, render.NodesTable(c))
	return 0
}

// cmdModels prints the runnable model catalog.
func (a *App) cmdModels(_ []string) int {
	models, err := a.Store.Models()
	if err != nil {
		fmt.Fprintf(a.Err, "relay: %v\n", err)
		return 1
	}
	fmt.Fprint(a.Out, render.ModelsTable(models))
	return 0
}

// cmdStatus prints a one-glance cluster summary.
func (a *App) cmdStatus(_ []string) int {
	c, err := a.Store.Snapshot()
	if err != nil {
		fmt.Fprintf(a.Err, "relay: %v\n", err)
		return 1
	}
	fmt.Fprint(a.Out, render.Status(c))
	return 0
}

// cmdRun routes a model through the explainable scheduler. With --explain it
// prints the full candidate evaluation; otherwise the concise verdict.
func (a *App) cmdRun(args []string) int {
	explain := false
	var model string
	for _, arg := range args {
		switch arg {
		case "--explain", "-e":
			explain = true
		default:
			if model == "" {
				model = arg
			}
		}
	}
	if model == "" {
		fmt.Fprintln(a.Err, "usage: relay run <model> [--explain]")
		return 2
	}

	c, err := a.Store.Snapshot()
	if err != nil {
		fmt.Fprintf(a.Err, "relay: %v\n", err)
		return 1
	}
	models, _ := a.Store.Models()
	cached, _ := a.Store.Cached()

	var want types.Model
	found := false
	for _, m := range models {
		if m.Name == model {
			want, found = m, true
			break
		}
	}
	if !found {
		fmt.Fprintf(a.Err, "relay: unknown model %q (try: relay models)\n", model)
		return 1
	}

	decision := scheduler.Schedule(c, want, cached)

	if explain {
		fmt.Fprint(a.Out, render.Explanation(decision))
		return 0
	}
	fmt.Fprint(a.Out, render.Placement(decision))
	if decision.Selected == nil {
		return 1
	}

	// Remote execution isn't wired up yet: Relay can only hand the terminal to
	// a runtime on the machine it's running on. When the scheduler picks
	// another node, report that rather than silently doing nothing.
	local := agent.LocalNode().Name
	if decision.Selected.Name != local {
		fmt.Fprintf(a.Err, "relay: selected node %q is remote; remote exec is not implemented yet.\n", decision.Selected.Name)
		fmt.Fprintf(a.Err, "       run this on %s, or use 'relay run %s --explain' to see placement.\n", decision.Selected.Name, model)
		return 1
	}

	adapter := rt.ByName(decision.Runtime)
	if adapter == nil {
		fmt.Fprintf(a.Err, "relay: no adapter for runtime %q\n", decision.Runtime)
		return 1
	}

	// Hand the terminal to the runtime. This blocks until the user exits the
	// interactive session, then returns control to the shell.
	if err := adapter.Run(types.Job{Model: want.Name, Runtime: decision.Runtime, Node: decision.Selected.Name}); err != nil {
		fmt.Fprintf(a.Err, "relay: session ended: %v\n", err)
		return 1
	}
	return 0
}

// cmdWatch renders the dashboard on a loop until interrupted.
func (a *App) cmdWatch(_ []string) int {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	defer signal.Stop(sigc)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	draw := func() {
		c, err := a.Store.Snapshot()
		if err != nil {
			fmt.Fprintf(a.Err, "relay: %v\n", err)
			return
		}
		fmt.Fprint(a.Out, render.ClearScreen())
		fmt.Fprint(a.Out, render.Dashboard(c))
		fmt.Fprintf(a.Out, "\n%s   %s\n", render.Timestamp(time.Now()), "Ctrl-C to exit")
	}

	draw()
	for {
		select {
		case <-sigc:
			fmt.Fprintln(a.Out)
			return 0
		case <-ticker.C:
			draw()
		}
	}
}
