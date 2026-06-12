package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	"relay/internal/agent"
	"relay/internal/controller"
	"relay/internal/render"
	rt "relay/internal/runtime"
	"relay/internal/scheduler"
	"relay/internal/types"
)

// cmdController runs the cluster controller: the HTTP registry agents join and
// heartbeat to, and the source `relay nodes/status/watch` read when pointed at
// it via RELAY_CONTROLLER. It blocks until interrupted.
func (a *App) cmdController(args []string) int {
	addr := flagValue(args, "--listen", ":7777")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// When listening on all interfaces (a bare ":port"), fill the host in the
	// hints with this machine's LAN IP so the printed commands are copy-paste
	// ready. Fall back to a placeholder if we can't determine it.
	hint := addr
	if strings.HasPrefix(addr, ":") {
		if ip := lanIP(); ip != "" {
			hint = ip + addr
		} else {
			hint = "<this-host>" + addr
		}
	}
	fmt.Fprintf(a.Out, "Relay controller listening on %s (Ctrl-C to stop).\n", addr)
	fmt.Fprintf(a.Out, "Agents: relay join --controller %s\n", hint)
	fmt.Fprintf(a.Out, "Read:   RELAY_CONTROLLER=%s relay nodes\n", hint)

	if err := controller.Serve(ctx, addr); err != nil {
		fmt.Fprintf(a.Err, "relay: controller: %v\n", err)
		return 1
	}
	fmt.Fprintln(a.Out, "\nController stopped.")
	return 0
}

// cmdJoin discovers this machine and joins it to the cluster. With a controller
// configured (--controller addr, or RELAY_CONTROLLER), it registers and then
// heartbeats in the foreground until interrupted — the agent daemon. Without
// one, it just prints what it discovered, so `relay join` is still useful for
// inspecting a host before any controller exists.
func (a *App) cmdJoin(args []string) int {
	addr := flagValue(args, "--controller", os.Getenv("RELAY_CONTROLLER"))

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

	if addr == "" {
		fmt.Fprintln(a.Out, "\nThis machine is ready to join. Point it at a controller:")
		fmt.Fprintln(a.Out, "  relay join --controller <host:port>   (or set RELAY_CONTROLLER)")
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Fprintln(a.Out)
	if err := agent.Serve(ctx, addr, a.Out); err != nil {
		fmt.Fprintf(a.Err, "relay: %v\n", err)
		return 1
	}
	return 0
}

// lanIP reports this machine's primary outbound IPv4, for printing a
// copy-pasteable controller address. It opens a UDP socket toward a public
// address — no packets are actually sent — and reads back the local endpoint
// the OS would route through. Returns "" if it can't be determined.
func lanIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.String()
	}
	return ""
}

// flagValue returns the argument following name, or def if name is absent. Tiny
// helper for the handful of flags here — not worth a flag package.
func flagValue(args []string, name, def string) string {
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return def
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
