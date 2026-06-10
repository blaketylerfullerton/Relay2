package store

import (
	"relay/internal/controller"
	"relay/internal/types"
)

// ControllerStore is the multi-node backend: it answers cluster questions by
// reading a live controller over HTTP. It is the production swap-in for
// LocalStore that the Store interface was designed to allow — the CLI,
// scheduler, and renderer are unchanged; they just see N nodes instead of one.
type ControllerStore struct {
	client *controller.Client
}

// NewController returns a ControllerStore talking to the controller at addr.
func NewController(addr string) *ControllerStore {
	return &ControllerStore{client: controller.NewClient(addr)}
}

// Snapshot fetches the whole fabric from the controller.
func (c *ControllerStore) Snapshot() (types.Cluster, error) {
	view, err := c.client.Cluster()
	if err != nil {
		return types.Cluster{}, err
	}
	// No measured peer links yet (network benchmarking is a later phase), so
	// the dashboard's Network section stays hidden until edges exist.
	return types.Cluster{Nodes: view.Nodes, Jobs: view.Jobs}, nil
}

// Models returns the deduplicated catalog across every node in the cluster.
func (c *ControllerStore) Models() ([]types.Model, error) {
	view, err := c.client.Cluster()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []types.Model
	for _, models := range view.Models {
		for _, m := range models {
			if seen[m.Name] {
				continue
			}
			seen[m.Name] = true
			out = append(out, m)
		}
	}
	return out, nil
}

// Cached reports which models are already resident on which nodes, keyed
// "node/model" — the format scheduler.Schedule expects. The controller's
// per-node model index is exactly this information.
func (c *ControllerStore) Cached() (map[string]bool, error) {
	view, err := c.client.Cluster()
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for node, models := range view.Models {
		for _, m := range models {
			out[node+"/"+m.Name] = true
		}
	}
	return out, nil
}
