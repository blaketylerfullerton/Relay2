// Package controller is the cluster brain: the process agents register with
// and heartbeat to, and the source of truth a CLI reads the whole fabric from.
//
// The spec eventually wants gRPC-over-TLS with auth and a SQLite registry. This
// first cut is deliberately smaller: HTTP+JSON (no new dependencies, same style
// as internal/runtime/probe.go) over an in-memory registry that is rebuilt from
// heartbeats anyway. The seam that matters is the wire protocol and the
// store.Store client, not the storage engine — those can harden later without
// the CLI, scheduler, or renderer noticing.
//
// TODO(next phase): TLS, node authentication, and gRPC. None are present here.
package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	"relay/internal/types"
)

// Registration is what an agent sends to join and to keep itself alive. It is
// the agent.Discovery shape (node + detected runtimes + local models) plus the
// jobs currently running on that node. The same body serves both /register and
// /heartbeat — registration is just the first heartbeat.
type Registration struct {
	Node     types.Node    `json:"node"`
	Runtimes []string      `json:"runtimes"`
	Models   []types.Model `json:"models"`
	Jobs     []types.Job   `json:"jobs"`
}

// ClusterView is the controller's answer to "what does the whole fabric look
// like right now?". It is types.Cluster plus a per-node model index so a client
// can derive the cluster-wide cache map (which node already has which model)
// without a second round trip.
type ClusterView struct {
	Nodes  []types.Node             `json:"nodes"`
	Jobs   []types.Job              `json:"jobs"`
	Models map[string][]types.Model `json:"models"` // keyed by node name
}

// Liveness thresholds, expressed as multiples of the heartbeat interval. A node
// that has missed a couple of beats is degraded; one that has gone quiet for
// longer is evicted entirely so it disappears from `relay nodes`/`watch`.
const (
	// HeartbeatInterval is how often agents are expected to report. Exported so
	// the agent loop and the reaper agree on the same cadence.
	HeartbeatInterval = 3 * time.Second

	degradedAfter = 2 * HeartbeatInterval
	evictAfter    = 4 * HeartbeatInterval
)

type entry struct {
	reg      Registration
	lastSeen time.Time
}

// Registry is the in-memory node table. Safe for concurrent use: heartbeats
// arrive on HTTP handler goroutines while the reaper and readers run alongside.
type Registry struct {
	mu    sync.Mutex
	nodes map[string]*entry
	now   func() time.Time // injectable clock for tests
}

// NewRegistry returns an empty registry using the wall clock.
func NewRegistry() *Registry {
	return &Registry{nodes: make(map[string]*entry), now: time.Now}
}

// Upsert records a registration/heartbeat, stamping it with the current time.
func (r *Registry) Upsert(reg Registration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes[reg.Node.Name] = &entry{reg: reg, lastSeen: r.now()}
}

// View returns the current cluster, with each node's Health and LastSeen
// recomputed from how long ago it last reported. Nodes past the eviction
// threshold are dropped. Output is sorted by node name so `relay watch` renders
// a stable, non-flickering order.
func (r *Registry) View() ClusterView {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	view := ClusterView{Models: map[string][]types.Model{}}

	for name, e := range r.nodes {
		age := now.Sub(e.lastSeen)
		if age >= evictAfter {
			continue
		}
		n := e.reg.Node
		n.LastSeen = e.lastSeen
		switch {
		case age >= degradedAfter:
			n.Health = types.HealthDegraded
		default:
			n.Health = types.HealthOnline
		}
		view.Nodes = append(view.Nodes, n)
		view.Jobs = append(view.Jobs, e.reg.Jobs...)
		view.Models[name] = e.reg.Models
	}

	sort.Slice(view.Nodes, func(i, j int) bool {
		return view.Nodes[i].Name < view.Nodes[j].Name
	})
	return view
}

// reap permanently removes nodes that have been silent past evictAfter, so the
// map doesn't grow without bound as machines come and go.
func (r *Registry) reap() {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	for name, e := range r.nodes {
		if now.Sub(e.lastSeen) >= evictAfter {
			delete(r.nodes, name)
		}
	}
}

// Server is the controller's HTTP handler over a Registry.
type Server struct {
	reg *Registry
	mux *http.ServeMux
}

// NewServer wires the routes over reg.
func NewServer(reg *Registry) *Server {
	s := &Server{reg: reg, mux: http.NewServeMux()}
	s.mux.HandleFunc("/v1/register", s.handleHeartbeat)
	s.mux.HandleFunc("/v1/heartbeat", s.handleHeartbeat)
	s.mux.HandleFunc("/v1/cluster", s.handleCluster)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// handleHeartbeat serves both /register and /heartbeat: registration is simply
// the first heartbeat, so they share an upsert.
func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var reg Registration
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		http.Error(w, "bad registration: "+err.Error(), http.StatusBadRequest)
		return
	}
	if reg.Node.Name == "" {
		http.Error(w, "registration missing node name", http.StatusBadRequest)
		return
	}
	s.reg.Upsert(reg)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleCluster(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.reg.View())
}

// Serve runs the controller on addr until ctx is cancelled, ticking the reaper
// in the background. It blocks; callers run it in the foreground (relay
// controller) and cancel ctx on interrupt.
func Serve(ctx context.Context, addr string) error {
	reg := NewRegistry()
	srv := &http.Server{Addr: addr, Handler: NewServer(reg)}

	go func() {
		t := time.NewTicker(HeartbeatInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				reg.reap()
			}
		}
	}()

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
