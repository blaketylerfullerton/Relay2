package store

import (
	"relay/internal/agent"
	rt "relay/internal/runtime"
	"relay/internal/types"
)

// LocalStore is the real, non-mock backend. It reports the actual machine the
// CLI runs on: real hardware via the agent, real models and running jobs via
// the detected runtimes.
//
// Phase 0 has no controller and no peer transport, so the "cluster" is exactly
// one node — this host. When the controller/gRPC layer lands, LocalStore
// becomes a controller client and the rest of the system is unchanged.
type LocalStore struct{}

// NewLocal returns a LocalStore.
func NewLocal() *LocalStore { return &LocalStore{} }

func (l *LocalStore) Snapshot() (types.Cluster, error) {
	node := agent.LocalNode()

	// Running jobs come from any detected runtime that can report them.
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
			j.Node = node.Name // adapters don't know the node name
			jobs = append(jobs, j)
		}
	}

	// No peers yet, so no network edges. The dashboard renders the Network
	// section only when links exist.
	return types.Cluster{
		Nodes: []types.Node{node},
		Jobs:  jobs,
	}, nil
}

func (l *LocalStore) Models() ([]types.Model, error) {
	seen := map[string]bool{}
	var out []types.Model
	for _, r := range rt.Detected() {
		found, err := r.DiscoverModels()
		if err != nil {
			continue
		}
		for _, m := range found {
			if seen[m.Name] {
				continue
			}
			seen[m.Name] = true
			out = append(out, m)
		}
	}
	return out, nil
}

func (l *LocalStore) Cached() (map[string]bool, error) {
	// Every model present locally is, by definition, cached on this node.
	node := agent.LocalNode()
	models, err := l.Models()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(models))
	for _, m := range models {
		out[node.Name+"/"+m.Name] = true
	}
	return out, nil
}
