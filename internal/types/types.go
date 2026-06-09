// Package types defines the core data model for the Relay compute fabric.
//
// These are the shapes an agent reports and the scheduler reasons over.
// In Phase 0 they're populated by a mock store; later they come from real
// agents reporting hardware, utilization, and peer latency over the WAN.
package types

import "time"

// Health describes whether a node is reachable and reporting.
type Health string

const (
	HealthOnline   Health = "online"
	HealthDegraded Health = "degraded"
	HealthOffline  Health = "offline"
)

// Node is a single machine that has joined the cluster.
type Node struct {
	Name      string  // human-friendly id, e.g. "office-west"
	GPU       string  // accelerator label, e.g. "RTX 4090", "H100"
	Runtime   string  // inference server, e.g. "vLLM", "llama.cpp"
	VRAMTotal int     // total VRAM in GB
	VRAMFree  int     // free VRAM in GB
	Util      float64 // current GPU utilization, 0.0–1.0
	Health    Health
	LastSeen  time.Time
}

// Job is a model currently scheduled onto a node.
type Job struct {
	ID      string // stable handle for Stop/inspect, e.g. "job-7f3a"
	Model   string // e.g. "llama3", "deepseek-r1"
	Node    string // name of the node running it
	Runtime string // adapter executing it, e.g. "ollama", "vllm"
}

// Link is a measured network edge between two peers.
type Link struct {
	A   string // node name
	B   string // node name
	RTT time.Duration
}

// Cluster is a point-in-time snapshot of the whole fabric.
type Cluster struct {
	Nodes []Node
	Jobs  []Job
	Links []Link
}

// Model is an entry in the catalog of runnable models.
type Model struct {
	Name   string
	Params string // e.g. "8B", "70B"
	Size   int    // approximate VRAM footprint in GB
}
