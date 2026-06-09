// Package runtime defines the adapter boundary between Relay and the inference
// servers it orchestrates (Ollama, vLLM, llama.cpp, ...).
//
// This is the spec's central design principle: runtime-specific logic must
// never be scattered through the codebase. Everything Relay needs from an
// engine goes through the Runtime interface, and each engine lives in exactly
// one file. The scheduler and CLI know about "a runtime", never about Ollama.
package runtime

import "relay/internal/types"

// Status is an adapter's self-reported health.
type Status struct {
	Healthy bool
	Detail  string // human-readable note, e.g. "listening on :11434"
}

// Runtime is one inference engine as seen by Relay. Adapters are expected to
// be cheap to construct; real work happens in the methods.
type Runtime interface {
	// Name is the stable adapter id, e.g. "ollama".
	Name() string

	// Detect reports whether this runtime is installed and usable on the
	// current host. Used by the agent during discovery.
	Detect() bool

	// DiscoverModels lists models this runtime already has locally.
	DiscoverModels() ([]types.Model, error)

	// Run starts serving the job's model. In Phase 0 adapters describe the
	// command they would execute rather than launching inference.
	Run(job types.Job) error

	// Pull fetches a model the runtime does not yet have.
	Pull(model types.Model) error

	// Stop tears down a running job by id.
	Stop(jobID string) error

	// Health probes the live engine.
	Health() Status
}

// All returns the adapters Relay ships with, in scheduler-preference order.
func All() []Runtime {
	return []Runtime{
		&Ollama{},
		&VLLM{},
		&LlamaCPP{},
	}
}

// ByName returns the shipped adapter whose Name matches, or nil if none does.
// Used by the CLI to turn a scheduler's runtime choice back into an adapter.
func ByName(name string) Runtime {
	for _, r := range All() {
		if r.Name() == name {
			return r
		}
	}
	return nil
}

// Detected returns only the adapters that report Detect() == true on this host.
func Detected() []Runtime {
	var out []Runtime
	for _, r := range All() {
		if r.Detect() {
			out = append(out, r)
		}
	}
	return out
}
