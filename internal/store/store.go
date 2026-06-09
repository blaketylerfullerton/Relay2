// Package store is the source of cluster state.
//
// Phase 0 ships an in-memory mock so the CLI is fully driveable before any
// real agent exists. The Store interface is the seam: swap MockStore for a
// gRPC/HTTP client that talks to live agents without touching the commands.
package store

import (
	"time"

	"relay/internal/types"
)

// Store is anything that can answer questions about the cluster.
type Store interface {
	Snapshot() (types.Cluster, error)
	Models() ([]types.Model, error)
	// Cached reports which models are already resident on which nodes, keyed
	// by "node/model". The scheduler treats a cache hit as a major win.
	Cached() (map[string]bool, error)
}

// MockStore returns a fixed, plausible cluster. Values jitter slightly over
// time so `relay watch` looks alive.
type MockStore struct {
	start time.Time
}

// NewMock returns a MockStore seeded at the current time.
func NewMock() *MockStore {
	return &MockStore{start: time.Now()}
}

func (m *MockStore) Snapshot() (types.Cluster, error) {
	now := time.Now()
	// A tiny deterministic wobble so utilization breathes between frames.
	t := now.Sub(m.start).Seconds()
	wob := func(base, amp float64) float64 {
		v := base + amp*sineish(t, base)
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		return v
	}

	nodes := []types.Node{
		{Name: "office-west", GPU: "RTX 4090", Runtime: "vLLM",
			VRAMTotal: 24, VRAMFree: 14, Util: wob(0.41, 0.05),
			Health: types.HealthOnline, LastSeen: now},
		{Name: "mac-studio", GPU: "M3 Ultra", Runtime: "llama.cpp",
			VRAMTotal: 64, VRAMFree: 50, Util: wob(0.22, 0.04),
			Health: types.HealthOnline, LastSeen: now},
		{Name: "dgx-lab", GPU: "H100", Runtime: "TensorRT-LLM",
			VRAMTotal: 80, VRAMFree: 18, Util: wob(0.78, 0.06),
			Health: types.HealthOnline, LastSeen: now},
	}

	jobs := []types.Job{
		{Model: "deepseek-r1", Node: "dgx-lab"},
		{Model: "llama3", Node: "office-west"},
	}

	links := []types.Link{
		{A: "office-west", B: "dgx-lab", RTT: 43 * time.Millisecond},
		{A: "office-west", B: "mac-studio", RTT: 9 * time.Millisecond},
		{A: "dgx-lab", B: "mac-studio", RTT: 51 * time.Millisecond},
	}

	return types.Cluster{Nodes: nodes, Jobs: jobs, Links: links}, nil
}

func (m *MockStore) Models() ([]types.Model, error) {
	return []types.Model{
		{Name: "llama3", Params: "8B", Size: 6},
		{Name: "llama3:70b", Params: "70B", Size: 42},
		{Name: "deepseek-r1", Params: "32B", Size: 22},
		{Name: "mistral", Params: "7B", Size: 5},
		{Name: "qwen2.5-coder", Params: "14B", Size: 10},
	}, nil
}

// Cached reflects models already pulled onto nodes. Mirrors the running jobs
// so `relay run deepseek-r1` demonstrates a cache-hit placement.
func (m *MockStore) Cached() (map[string]bool, error) {
	return map[string]bool{
		"dgx-lab/deepseek-r1": true,
		"office-west/llama3":  true,
		"mac-studio/llama3":   true,
		"office-west/mistral": true,
	}, nil
}

// sineish is a cheap, dependency-free pseudo-oscillator in [-1,1].
func sineish(t, phase float64) float64 {
	x := t*0.6 + phase*10
	// Taylor-ish triangle wave; we only need motion, not accuracy.
	x = x - float64(int(x/6.283185)*6) // crude mod into a few periods
	if x > 3.14159 {
		return 1 - (x-3.14159)/3.14159*2
	}
	return -1 + x/3.14159*2
}
