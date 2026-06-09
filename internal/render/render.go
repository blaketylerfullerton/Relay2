// Package render turns a cluster snapshot into the terminal dashboard.
//
// All output goes through here so the look stays consistent across `nodes`,
// `watch`, and friends. Colors are raw ANSI so we carry no dependencies.
package render

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"relay/internal/types"
)

// ANSI helpers. NO_COLOR is honored by the caller flipping useColor.
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	cyan   = "\033[36m"
)

// barWidth is the number of cells in a utilization bar.
const barWidth = 8

// Dashboard renders the full cluster view shown by `relay watch`.
func Dashboard(c types.Cluster) string {
	var b strings.Builder

	b.WriteString(bold + "Relay Cluster" + reset + "\n")
	b.WriteString(dim + strings.Repeat("─", 36) + reset + "\n\n")

	// Nodes, widest-name aligned.
	nameW, gpuW := 0, 0
	for _, n := range c.Nodes {
		nameW = max(nameW, len(n.Name))
		gpuW = max(gpuW, len(n.GPU))
	}
	for _, n := range c.Nodes {
		pct := int(n.Util*100 + 0.5)
		b.WriteString(fmt.Sprintf("%-*s  %-*s  %s %3d%%\n",
			nameW, n.Name, gpuW, n.GPU, bar(n.Util), pct))
	}

	// Running jobs.
	if len(c.Jobs) > 0 {
		b.WriteString("\n" + bold + "Running Jobs" + reset + "\n\n")
		jobW := 0
		for _, j := range c.Jobs {
			jobW = max(jobW, len(j.Model))
		}
		for _, j := range c.Jobs {
			b.WriteString(fmt.Sprintf("%-*s  %s->%s  %s\n",
				jobW, j.Model, dim, reset, j.Node))
		}
	}

	// Network edges.
	if len(c.Links) > 0 {
		b.WriteString("\n" + bold + "Network" + reset + "\n\n")
		// Pre-format the "A <--> B" column so RTTs line up.
		type row struct{ pair, rtt string }
		rows := make([]row, 0, len(c.Links))
		pairW := 0
		for _, l := range c.Links {
			pair := fmt.Sprintf("%s <--> %s", l.A, l.B)
			pairW = max(pairW, len(pair))
			rows = append(rows, row{pair, fmt.Sprintf("%d ms", l.RTT.Milliseconds())})
		}
		for _, r := range rows {
			b.WriteString(fmt.Sprintf("%-*s   %s%s%s\n",
				pairW, r.pair, latencyColor(r.rtt), r.rtt, reset))
		}
	}

	return b.String()
}

// NodesTable renders the compact `relay nodes` listing. Column widths flex to
// the data so real (often long) hostnames and GPU labels stay aligned.
func NodesTable(c types.Cluster) string {
	var b strings.Builder
	nodes := append([]types.Node(nil), c.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })

	nameW, gpuW, rtW := len("NAME"), len("GPU"), len("RUNTIME")
	for _, n := range nodes {
		nameW = max(nameW, len(n.Name))
		gpuW = max(gpuW, len(n.GPU))
		rtW = max(rtW, len(n.Runtime))
	}

	b.WriteString(fmt.Sprintf("%s%-*s  %-*s  %-*s  %-6s %s%s\n",
		bold, nameW, "NAME", gpuW, "GPU", rtW, "RUNTIME", "FREE", "HEALTH", reset))
	for _, n := range nodes {
		b.WriteString(fmt.Sprintf("%-*s  %-*s  %-*s  %-6s %s\n",
			nameW, n.Name, gpuW, n.GPU, rtW, n.Runtime,
			fmt.Sprintf("%dGB", n.VRAMFree), healthDot(n.Health)))
	}
	return b.String()
}

// ModelsTable renders the `relay models` catalog.
func ModelsTable(models []types.Model) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s%-18s %-8s %s%s\n", bold, "MODEL", "PARAMS", "VRAM", reset))
	for _, m := range models {
		b.WriteString(fmt.Sprintf("%-18s %-8s %dGB\n", m.Name, m.Params, m.Size))
	}
	return b.String()
}

// bar draws a unicode utilization bar colored by load.
func bar(util float64) string {
	filled := int(util*float64(barWidth) + 0.5)
	if filled > barWidth {
		filled = barWidth
	}
	color := green
	switch {
	case util >= 0.75:
		color = red
	case util >= 0.5:
		color = yellow
	}
	return color + strings.Repeat("█", filled) + reset +
		dim + strings.Repeat("░", barWidth-filled) + reset
}

func latencyColor(rtt string) string {
	// rtt looks like "43 ms"; we just bucket on the leading number.
	var ms int
	fmt.Sscanf(rtt, "%d", &ms)
	switch {
	case ms >= 50:
		return yellow
	case ms >= 100:
		return red
	default:
		return green
	}
}

func healthDot(h types.Health) string {
	switch h {
	case types.HealthOnline:
		return green + "● online" + reset
	case types.HealthDegraded:
		return yellow + "● degraded" + reset
	default:
		return red + "● offline" + reset
	}
}

// ClearScreen returns the ANSI sequence to reset the cursor for watch frames.
func ClearScreen() string { return "\033[H\033[2J" }

// Timestamp is a small dim footer used by watch.
func Timestamp(t time.Time) string {
	return dim + "updated " + t.Format("15:04:05") + reset
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
