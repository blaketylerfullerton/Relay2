package controller

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"relay/internal/types"
)

// clock is a controllable time source for liveness tests.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func testReg(name string) Registration {
	return Registration{
		Node:   types.Node{Name: name, GPU: "H100", VRAMTotal: 80, VRAMFree: 40},
		Models: []types.Model{{Name: "llama3", Size: 6}},
	}
}

// TestRegisterThenView exercises the full wire path: an agent registers over
// HTTP via the Client, and a reader sees it as an online node in the view.
func TestRegisterThenView(t *testing.T) {
	reg := NewRegistry()
	ts := httptest.NewServer(NewServer(reg))
	defer ts.Close()

	client := NewClient(strings.TrimPrefix(ts.URL, "http://"))
	if err := client.Register(testReg("dgx-lab")); err != nil {
		t.Fatalf("register: %v", err)
	}

	view, err := client.Cluster()
	if err != nil {
		t.Fatalf("cluster: %v", err)
	}
	if len(view.Nodes) != 1 || view.Nodes[0].Name != "dgx-lab" {
		t.Fatalf("expected one node 'dgx-lab', got %+v", view.Nodes)
	}
	if view.Nodes[0].Health != types.HealthOnline {
		t.Fatalf("fresh node should be online, got %q", view.Nodes[0].Health)
	}
	if got := view.Models["dgx-lab"]; len(got) != 1 || got[0].Name != "llama3" {
		t.Fatalf("expected per-node model index for cache derivation, got %+v", got)
	}
}

// TestLiveness walks a node through online → degraded → evicted as its last
// heartbeat ages, and confirms a fresh heartbeat brings it back to online.
func TestLiveness(t *testing.T) {
	clk := &clock{t: time.Unix(1_700_000_000, 0)}
	reg := NewRegistry()
	reg.now = clk.now

	reg.Upsert(testReg("office-west"))

	if h := reg.View().Nodes[0].Health; h != types.HealthOnline {
		t.Fatalf("just-registered node should be online, got %q", h)
	}

	clk.advance(degradedAfter)
	if h := reg.View().Nodes[0].Health; h != types.HealthDegraded {
		t.Fatalf("node past degradedAfter should be degraded, got %q", h)
	}

	// A fresh heartbeat resets liveness back to online.
	reg.Upsert(testReg("office-west"))
	if h := reg.View().Nodes[0].Health; h != types.HealthOnline {
		t.Fatalf("node should be online again after heartbeat, got %q", h)
	}

	// Go quiet past the eviction threshold: it disappears from the view, and
	// reap removes it from the registry entirely.
	clk.advance(evictAfter)
	if n := len(reg.View().Nodes); n != 0 {
		t.Fatalf("evicted node should not appear in view, got %d nodes", n)
	}
	reg.reap()
	reg.mu.Lock()
	_, present := reg.nodes["office-west"]
	reg.mu.Unlock()
	if present {
		t.Fatal("reap should have removed the evicted node from the registry")
	}
}
