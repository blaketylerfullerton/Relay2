// Package scheduler decides which node runs a model, and — just as importantly
// — why. The spec is emphatic that the scheduler is Relay's long-term
// differentiator and that every decision must be explainable. So Schedule does
// not return a bare node; it returns a Decision that records each candidate it
// considered, whether the candidate was viable, and the reasoning behind the
// final pick. `relay run --explain` prints this verbatim.
package scheduler

import (
	"fmt"
	"sort"

	"relay/internal/types"
)

// Candidate is one node the scheduler evaluated.
type Candidate struct {
	Node    types.Node
	Viable  bool     // could this node run the model at all?
	Reasons []string // why it was/wasn't viable, in human terms
	score   float64  // internal ranking, higher is better
}

// Decision is the full, explainable result of a scheduling pass.
type Decision struct {
	Model      types.Model
	Selected   *types.Node // nil when nothing could fit
	Runtime    string      // adapter chosen on the selected node
	Candidates []Candidate // every node considered, best-first
	Summary    []string    // the headline reasons the winner won
}

// Schedule ranks nodes for model and returns an explainable Decision.
//
// Phase-0 signals (a subset of the spec's eventual list): VRAM headroom,
// whether the model is already cached, GPU utilization, and node health.
// The weighting is deliberately simple and legible — explainability beats
// cleverness here.
func Schedule(c types.Cluster, model types.Model, cached map[string]bool) Decision {
	d := Decision{Model: model}

	for _, n := range c.Nodes {
		cand := Candidate{Node: n}

		switch {
		case n.Health != types.HealthOnline:
			cand.Reasons = append(cand.Reasons, fmt.Sprintf("node is %s", n.Health))
		case n.VRAMFree < model.Size:
			cand.Reasons = append(cand.Reasons,
				fmt.Sprintf("insufficient VRAM (%dGB free < %dGB needed)", n.VRAMFree, model.Size))
		default:
			cand.Viable = true
			// Base score: free headroom after loading, normalized.
			headroom := float64(n.VRAMFree - model.Size)
			cand.score = headroom
			cand.Reasons = append(cand.Reasons,
				fmt.Sprintf("fits with %dGB to spare", n.VRAMFree-model.Size))

			// Idle GPUs are cheaper to schedule onto.
			cand.score += (1 - n.Util) * 20
			cand.Reasons = append(cand.Reasons,
				fmt.Sprintf("GPU utilization %d%%", int(n.Util*100+0.5)))

			// A cached model means no download — a big, legible win.
			if cached[key(n.Name, model.Name)] {
				cand.score += 100
				cand.Reasons = append(cand.Reasons, "model already cached")
			} else {
				cand.Reasons = append(cand.Reasons, "model not cached (would download)")
			}
		}

		d.Candidates = append(d.Candidates, cand)
	}

	// Best viable candidate first; non-viable sink to the bottom.
	sort.SliceStable(d.Candidates, func(i, j int) bool {
		a, b := d.Candidates[i], d.Candidates[j]
		if a.Viable != b.Viable {
			return a.Viable
		}
		return a.score > b.score
	})

	if len(d.Candidates) > 0 && d.Candidates[0].Viable {
		win := d.Candidates[0]
		node := win.Node
		d.Selected = &node
		d.Runtime = runtimeFor(node)
		d.Summary = win.Reasons
	}

	return d
}

// key namespaces the cache map by node+model.
func key(node, model string) string { return node + "/" + model }

// runtimeFor maps a node's engine label to the adapter Relay would drive. The
// node already reports the runtime it runs; we normalize it to an adapter id.
func runtimeFor(n types.Node) string {
	switch n.Runtime {
	case "vLLM":
		return "vllm"
	case "llama.cpp":
		return "llama.cpp"
	case "TensorRT-LLM":
		return "tensorrt-llm"
	default:
		return "ollama"
	}
}
