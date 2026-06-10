package agent

import (
	"context"
	"fmt"
	"io"
	"time"

	"relay/internal/controller"
	rt "relay/internal/runtime"
	"relay/internal/types"
)

// registration assembles this machine's current state for the controller: the
// full discovery (host, runtimes, local models) plus whatever jobs are running
// right now. It's recomputed each heartbeat so util/VRAM/jobs stay fresh.
func registration() controller.Registration {
	d := Inspect()
	return controller.Registration{
		Node:     d.Node,
		Runtimes: d.Runtimes,
		Models:   d.Models,
		Jobs:     runningJobs(d.Node.Name),
	}
}

// runningJobs asks every detected runtime that can report live models which
// ones it currently has loaded, tagging each with this node's name. Mirrors the
// logic LocalStore uses for a single-node snapshot.
func runningJobs(node string) []types.Job {
	var jobs []types.Job
	for _, r := range rt.Detected() {
		rl, ok := r.(rt.RunningLister)
		if !ok {
			continue
		}
		running, err := rl.Running()
		if err != nil {
			continue
		}
		for _, j := range running {
			j.Node = node
			jobs = append(jobs, j)
		}
	}
	return jobs
}

// Serve registers this machine with the controller at addr and heartbeats on
// the shared interval until ctx is cancelled. This is the daemon half of the
// agent the spec models on kubelet/tailscaled; `relay join` runs it in the
// foreground. Progress lines are written to out.
func Serve(ctx context.Context, addr string, out io.Writer) error {
	client := controller.NewClient(addr)

	if err := client.Register(registration()); err != nil {
		return fmt.Errorf("registering with controller: %w", err)
	}
	fmt.Fprintf(out, "Registered with controller %s as %q. Heartbeating every %s (Ctrl-C to leave).\n",
		addr, LocalNode().Name, controller.HeartbeatInterval)

	t := time.NewTicker(controller.HeartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(out, "\nLeaving cluster.")
			return nil
		case <-t.C:
			if err := client.Heartbeat(registration()); err != nil {
				// A blip shouldn't kill the agent: log and keep trying so the
				// node re-appears once the controller is back.
				fmt.Fprintf(out, "heartbeat failed: %v\n", err)
			}
		}
	}
}
