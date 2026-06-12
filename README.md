## Relay 2

### Overview
Relay is a WAN native runtime that turns geographically distributed heterogenious GPUs into a simple programmable AI compute fabric

**Relay is the way to distribute an entire AI system**

Relay schedules the entire DAG

One way to think about it

Docker: 
    - Package one application

Kubernetes:
    - Coordinate Thousands of containers

Ollama:
    - Run one model

Petals:
    - Split one model

Relay:
    - Coordinate an entire AI Infrustructure

Why this is different. 
    The hardest part is not about splitting tranformer layers.
    The hardest part is building a scheduler that can asnwer:
        - Which node should run this
        - Should i replicate KV cahce
        - is WAN latency worth paying 
        - Should speculative deocding happen locally
        - Is bandwhich saturated


---

### Install

Install the `relay` binary on any machine. No Go or source checkout required —
this downloads a prebuilt binary for the machine's OS/arch:

```bash
curl -fsSL https://raw.githubusercontent.com/blaketylerfullerton/Relay2/main/scripts/install.sh | sh
```

It lands in `~/.local/bin/relay` (override with `RELAY_BIN_DIR`). Make sure that
directory is on your `PATH`. Supported targets: macOS (arm64/amd64) and Linux
(amd64/arm64).

Then:

```bash
relay join      # inspect this machine (add --controller <addr> to join a cluster)
relay nodes     # list machines
relay version   # print the installed version
```

To wire multiple machines together, see **Running a cluster** below.

### Running a cluster

A Relay cluster is one **controller** (the registry every machine reports to)
and any number of **agents** (the machines that actually run models). The
controller can live on one of the agent machines — it's lightweight.

**1. Start the controller** on one machine:

```bash
relay controller            # listens on :7777
# or pin a port: relay controller --listen :9000
```

On startup it prints copy-pasteable commands with this machine's LAN IP:

```
Relay controller listening on :7777 (Ctrl-C to stop).
Agents: relay join --controller 192.168.0.76:7777
Read:   RELAY_CONTROLLER=192.168.0.76:7777 relay nodes
```

**2. Join each machine** (run this on every box that should run models —
including the controller box itself):

```bash
relay join --controller 192.168.0.76:7777
```

`join` registers the host's real GPU/VRAM and detected models, then heartbeats
in the foreground until you Ctrl-C. To keep it running after logout, launch it
under `tmux`, `nohup`, or a `launchd`/`systemd` unit.

**3. Read the cluster** from anywhere that can reach the controller. Point the
read commands at it with `RELAY_CONTROLLER`:

```bash
export RELAY_CONTROLLER=192.168.0.76:7777
relay nodes                  # every joined machine
relay status                 # aggregate health + VRAM
relay watch                  # live dashboard; nodes drop ~12s after a box dies
relay run <model> --explain  # scheduler ranks across all nodes
```

Notes:

- **Same network only.** The printed IP is reachable from the controller's LAN.
  On a different subnet/VPN, use whatever address that machine can actually
  reach the controller on.
- **No auth or TLS yet** — this is plaintext HTTP for a trusted network. Don't
  expose the controller port to the internet.
- **Try it on one machine:** give a second `join` a distinct name to simulate a
  second node — `RELAY_NODE_NAME=mac-studio relay join --controller 127.0.0.1:7777`.

### Updating

To bring a machine to the latest code:

```bash
relay update
```

`relay update` picks its mode automatically:

- **Release mode (default):** downloads the newest prebuilt binary for this
  OS/arch and swaps it in place. No Go needed. This is what your fleet uses.
- **Source mode:** on a machine that has a Relay git checkout (`$RELAY_SRC`) and
  Go installed, it runs `git pull` + `go build` instead. This is for dev boxes.

Pass `--force` to reinstall even when already up to date.

### Distribution (how updates ship)

Updates flow through GitHub Releases — you never hand-copy binaries.

```
dev machine                GitHub                       fleet
-----------                ------                       -----
git push  ──────────▶  Actions cross-compiles  ──────▶  relay update
                       darwin/linux × arm64/amd64       (downloads latest)
                       publishes "latest" release
```

1. **Push to `main`.** The `release` workflow
   (`.github/workflows/release.yml`) cross-compiles `relay` for every supported
   OS/arch and publishes them to a rolling **`latest`** GitHub Release. No tags,
   no manual builds.
2. **On each machine, run `relay update`.** It pulls the freshly built binary
   from that release.

The version stamped into release binaries is `main-<short-sha>`, so
`relay version` and `relay update`'s "already up to date" check reflect the
exact commit a machine is running.

**Developing on a machine?** Build straight from your checkout instead of
downloading:

```bash
export RELAY_SRC="$HOME/path/to/Relay2"   # your clone
relay update                              # git pull + go build, in place
```

Or install from source explicitly: `RELAY_FROM_SOURCE=1 ./scripts/install.sh`.

---

Phase 0: Building the scheduler not the inference. We are NOt trying to invednt a new distrubuteed transformer architecture. We are building a runtime that orchestrates the existing inference servers.

```
relay-agent install

Machine A
- RTX 4090
- vLLM
- 24GB VRAM

Machine B
- Mac Studio
- llama.cpp
- 64GB Unified Memory

Machine C
- H100
- TensorRT-LLM
```

Each agent periodically reports:
Hardware
Available RAM
Current Utilization
RTT to peers
Bandwitdh estimates
Running models
Health status

At tjhis point, relay is basically a GPU inventory system

Phase 1: "SSH for AI"
We just type `relay nodes`
Ouput:
```
NAME            GPU     FREE
office-west     H100    716GB
home-pc         4090    18GB
```

Then
```
relay exec home-pc ollama run llama3
```
or
```
relay run llama-70b
```

Phase 2: Automatic placement
simply say:
```
relay run deepseek-r1
```

Scheduler decides:
- Enough VRAM?
- Lowest latency?
- already cached?
- GPU current idle
- Cheapest nodes?

Starting to look like kubernetes clustering

Phase 3: DAG Execution
```
PDF Upload

    │

Embedding Model

    │

Retriever

    │

Reranker

    │

LLM

    │

Voice Synthesizer
```