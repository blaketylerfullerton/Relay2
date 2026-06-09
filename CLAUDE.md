# CLAUDE.md

## Relay

Relay is a research-driven systems project exploring a simple idea:

> AI compute should behave like a cluster, not a collection of disconnected machines.

Modern organizations already own heterogeneous AI hardware:

* Consumer NVIDIA GPUs
* DGX systems
* Apple Silicon machines
* Workstations
* Servers
* Future accelerators

Today each machine is managed independently through tools like Ollama, vLLM, TensorRT-LLM, llama.cpp, and SGLang.

Relay's purpose is **not** to replace these runtimes.

Relay is the control plane that unifies them.

---

# Philosophy

Relay should feel more like:

* Docker
* Kubernetes
* Tailscale

than:

* Another inference engine
* Another model server
* Another chatbot framework

The user should never need to know which runtime actually executes their workload.

Instead of:

```bash
ollama run llama3
```

or

```bash
python -m vllm.entrypoints.openai.api_server ...
```

the user simply writes:

```bash
relay run llama3
```

Relay determines:

* Which machine should execute
* Which runtime should be used
* Whether the model already exists
* Whether it should be downloaded
* How to recover from failures

---

# Core Design Principle

**Relay schedules AI workloads.**

It does not implement transformer inference itself.

Relay delegates execution to existing runtimes through adapters.

```
User
  |
  v
Relay CLI
  |
  v
Relay Scheduler
  |
  +-------------------------+
  |            |            |
  v            v            v
Ollama       vLLM      llama.cpp
```

---

# Architecture

## Components

### relay-agent

Runs on every machine.

Responsibilities:

* Hardware discovery
* GPU inventory
* Runtime detection
* Model discovery
* Health reporting
* Job execution
* Network benchmarking

The agent should remain lightweight.

It should resemble a kubelet or tailscaled daemon.

---

### relay-controller

Maintains global cluster state.

Responsibilities:

* Scheduler
* Authentication
* Node registry
* Model registry
* Cluster topology
* Job orchestration

The controller is the brain.

---

### relay-cli

The primary user interface.

The terminal is the product.

A web dashboard is intentionally out of scope for V1.

---

# Initial Scope

V1 goals:

* Discover machines
* Detect installed runtimes
* Detect installed models
* Execute workloads on the best node
* Provide cluster visibility

Supported commands:

```bash
relay join
relay nodes
relay models
relay run <model>
relay status
relay watch
```

Example:

```bash
$ relay run llama3

Scheduler:
✓ Selected node: office-west
✓ Runtime: Ollama

Connecting...

>
```

---

# Explicit Non-Goals

Do NOT build these in the first version:

* Distributed transformer layer execution
* WAN tensor parallelism
* Distributed KV caches
* Speculative decoding
* Federated training
* Multi-cloud orchestration
* Kubernetes replacement
* Web dashboard
* User accounts
* Billing
* Enterprise RBAC

These may become future research areas, but they are not required to validate the project.

---

# Technology Stack

Language: Go

Reasoning:

* Single static binaries
* Excellent networking support
* Simple deployment
* Cross-platform compilation
* Goroutines map naturally to concurrent agents

Frontend:

* None for V1

Future:

* React
* TypeScript

Communication:

* gRPC over TLS

Database:

* SQLite initially
* PostgreSQL when necessary

---

# Runtime Adapter Model

Relay should never contain runtime-specific logic scattered throughout the codebase.

Use adapters.

```go
type Runtime interface {
    Name() string
    DiscoverModels() error
    Run(Job) error
    Pull(Model) error
    Stop(JobID) error
    Health() Status
}
```

Supported adapters:

* Ollama
* vLLM
* llama.cpp

Future:

* TensorRT-LLM
* SGLang
* Additional runtimes

---

# Scheduler Philosophy

The scheduler is the primary source of long-term differentiation.

Scheduling decisions should eventually consider:

* Available VRAM
* Existing model cache
* GPU utilization
* Network latency
* Bandwidth
* Queue depth
* Historical throughput
* Trust boundaries

The scheduler should always be explainable.

Future CLI:

```bash
relay run qwen3 --explain
```

Example:

```
Candidate Nodes

office-west
✗ insufficient VRAM

dgx-lab
✓ selected

Reason:
- model already cached
- lowest estimated completion time
- GPU utilization 18%
```

---

# Code Style

Prefer:

* Simplicity
* Small packages
* Clear interfaces
* Composition over inheritance
* Explicit behavior

Avoid:

* Premature abstractions
* Large frameworks
* Hidden magic
* Global state

---

# Project Vision

Docker standardized containers.

Kubernetes standardized clusters.

Relay aims to standardize heterogeneous AI compute.

Long-term, Relay should make geographically distributed AI infrastructure appear as a single programmable system.

The initial version should accomplish this with as little complexity as possible.
