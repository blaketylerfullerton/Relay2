package render

import (
	"fmt"
	"strings"

	"relay/internal/scheduler"
	"relay/internal/types"
)

// Placement renders the concise result of `relay run` (no --explain): the
// scheduler's verdict in the spec's checkmark style.
func Placement(d scheduler.Decision) string {
	var b strings.Builder
	if d.Selected == nil {
		b.WriteString(red + "✗ no node can run " + d.Model.Name + reset + "\n")
		return b.String()
	}
	b.WriteString(bold + "Scheduler:" + reset + "\n")
	b.WriteString(fmt.Sprintf("%s✓%s Selected node: %s\n", green, reset, d.Selected.Name))
	b.WriteString(fmt.Sprintf("%s✓%s Runtime: %s\n", green, reset, d.Runtime))
	b.WriteString("\nConnecting...\n\n> ")
	return b.String()
}

// Explanation renders `relay run --explain`: every candidate considered, why
// each was or wasn't viable, and the winner's reasoning — the spec's
// explainable-scheduler view.
func Explanation(d scheduler.Decision) string {
	var b strings.Builder
	b.WriteString(bold + "Candidate Nodes" + reset + "\n\n")

	for _, c := range d.Candidates {
		mark := red + "✗" + reset
		if c.Viable {
			mark = green + "✓" + reset
		}
		selected := ""
		if d.Selected != nil && c.Node.Name == d.Selected.Name {
			selected = bold + "  <- selected" + reset
		}
		b.WriteString(fmt.Sprintf("%s %s%s\n", mark, c.Node.Name, selected))
		for _, r := range c.Reasons {
			b.WriteString(fmt.Sprintf("    %s- %s%s\n", dim, r, reset))
		}
		b.WriteString("\n")
	}

	if d.Selected != nil {
		b.WriteString(bold + "Decision" + reset + "\n")
		b.WriteString(fmt.Sprintf("  run %s on %s via %s\n",
			d.Model.Name, d.Selected.Name, d.Runtime))
	} else {
		b.WriteString(red + "No viable node for " + d.Model.Name + reset + "\n")
	}
	return b.String()
}

// Status renders the `relay status` cluster summary: a one-glance health and
// capacity readout across the fabric.
func Status(c types.Cluster) string {
	var b strings.Builder
	online, total := 0, len(c.Nodes)
	var vramTotal, vramFree int
	for _, n := range c.Nodes {
		if n.Health == types.HealthOnline {
			online++
		}
		vramTotal += n.VRAMTotal
		vramFree += n.VRAMFree
	}

	b.WriteString(bold + "Relay Status" + reset + "\n")
	b.WriteString(dim + strings.Repeat("─", 36) + reset + "\n\n")
	b.WriteString(fmt.Sprintf("  nodes      %s%d/%d online%s\n", green, online, total, reset))
	b.WriteString(fmt.Sprintf("  vram       %dGB free of %dGB\n", vramFree, vramTotal))
	b.WriteString(fmt.Sprintf("  jobs       %d running\n", len(c.Jobs)))
	b.WriteString(fmt.Sprintf("  links      %d measured\n", len(c.Links)))
	return b.String()
}

// Discovery renders what an agent learned about the local machine, used by
// `relay join`.
func Discovery(node types.Node, runtimes []string, models []types.Model) string {
	var b strings.Builder
	rts := "none detected"
	if len(runtimes) > 0 {
		rts = strings.Join(runtimes, ", ")
	}
	b.WriteString(fmt.Sprintf("  hostname ........ %s\n", node.Name))
	b.WriteString(fmt.Sprintf("  accelerator ..... %s\n", node.GPU))
	if node.VRAMTotal > 0 {
		b.WriteString(fmt.Sprintf("  memory .......... %dGB\n", node.VRAMTotal))
	}
	b.WriteString(fmt.Sprintf("  runtimes ........ %s\n", rts))
	b.WriteString(fmt.Sprintf("  local models .... %d\n", len(models)))
	return b.String()
}
