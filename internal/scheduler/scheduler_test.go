package scheduler

import (
	"testing"
	"time"

	"relay/internal/types"
)

func node(name string, vramFree int, util float64, health types.Health) types.Node {
	return types.Node{
		Name: name, GPU: "test", Runtime: "Ollama",
		VRAMTotal: 80, VRAMFree: vramFree, Util: util,
		Health: health, LastSeen: time.Now(),
	}
}

func TestSchedule_PicksViableNode(t *testing.T) {
	c := types.Cluster{Nodes: []types.Node{
		node("small", 4, 0.1, types.HealthOnline),
		node("big", 40, 0.1, types.HealthOnline),
	}}
	model := types.Model{Name: "llama3", Size: 6}

	d := Schedule(c, model, nil)
	if d.Selected == nil {
		t.Fatal("expected a selection, got none")
	}
	if d.Selected.Name != "big" {
		t.Fatalf("expected 'big' (only node with enough VRAM), got %q", d.Selected.Name)
	}
}

func TestSchedule_NothingFits(t *testing.T) {
	c := types.Cluster{Nodes: []types.Node{
		node("small", 4, 0.1, types.HealthOnline),
	}}
	model := types.Model{Name: "llama3:70b", Size: 42}

	d := Schedule(c, model, nil)
	if d.Selected != nil {
		t.Fatalf("expected nil selection when nothing fits, got %q", d.Selected.Name)
	}
	if len(d.Candidates) != 1 || d.Candidates[0].Viable {
		t.Fatal("expected the single candidate to be non-viable")
	}
}

func TestSchedule_OfflineExcluded(t *testing.T) {
	c := types.Cluster{Nodes: []types.Node{
		node("offline-big", 80, 0.0, types.HealthOffline),
		node("online-ok", 20, 0.5, types.HealthOnline),
	}}
	model := types.Model{Name: "mistral", Size: 5}

	d := Schedule(c, model, nil)
	if d.Selected == nil || d.Selected.Name != "online-ok" {
		t.Fatalf("expected 'online-ok'; offline node must be excluded, got %v", d.Selected)
	}
}

func TestSchedule_CacheHitOutranksIdle(t *testing.T) {
	// Both nodes fit. "cached" has more load and less headroom, but already has
	// the model — the cache-hit bonus should make it win, and the explanation
	// should say so.
	c := types.Cluster{Nodes: []types.Node{
		node("idle", 60, 0.05, types.HealthOnline),
		node("cached", 20, 0.60, types.HealthOnline),
	}}
	model := types.Model{Name: "deepseek-r1", Size: 10}
	cached := map[string]bool{"cached/deepseek-r1": true}

	d := Schedule(c, model, cached)
	if d.Selected == nil || d.Selected.Name != "cached" {
		t.Fatalf("expected cache-hit node 'cached' to win, got %v", d.Selected)
	}
	if !hasReason(d.Summary, "model already cached") {
		t.Fatalf("expected summary to explain the cache hit, got %v", d.Summary)
	}
}

func hasReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}
