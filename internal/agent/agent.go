// Package agent is the per-machine side of Relay: the lightweight daemon-style
// component that discovers what a host actually has and reports it inward. The
// spec models it on kubelet/tailscaled — it stays small and does no scheduling.
//
// Phase 0 implements the discovery half (hardware, runtimes, models) so that
// `relay join` reflects the real local machine instead of a canned demo. The
// reporting half (gRPC to the controller) is a later phase; Report() returns
// the struct that transport will eventually carry.
package agent

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	rt "relay/internal/runtime"
	"relay/internal/types"
)

// Discovery is everything a single agent learns about its host.
type Discovery struct {
	Node     types.Node
	Runtimes []string      // detected adapter names
	Models   []types.Model // models found across detected runtimes
}

// LocalNode reports this machine's hardware, free VRAM, and utilization. It is
// the cheap path used on every cluster snapshot: no runtime model discovery,
// just accelerator probing. This is the real node that replaces a mock entry.
func LocalNode() types.Node {
	hw := detectAccelerator()

	primary := "none"
	if detected := rt.Detected(); len(detected) > 0 {
		names := make([]string, 0, len(detected))
		for _, r := range detected {
			names = append(names, r.Name())
		}
		primary = primaryRuntime(names)
	}

	return types.Node{
		Name:      hostName(),
		GPU:       hw.name,
		Runtime:   primary,
		VRAMTotal: hw.vramTotal,
		VRAMFree:  hw.vramFree,
		Util:      hw.util,
		Health:    types.HealthOnline,
		LastSeen:  time.Now(),
	}
}

// Inspect is the full discovery used by `relay join`: LocalNode plus the
// detected runtimes and the models they report.
func Inspect() Discovery {
	node := LocalNode()

	var runtimes []string
	var models []types.Model
	for _, r := range rt.Detected() {
		runtimes = append(runtimes, r.Name())
		if found, err := r.DiscoverModels(); err == nil {
			models = append(models, found...)
		}
	}

	return Discovery{Node: node, Runtimes: runtimes, Models: models}
}

// hardware is a best-effort accelerator readout.
type hardware struct {
	name      string
	vramTotal int     // GB
	vramFree  int     // GB
	util      float64 // 0..1
}

// detectAccelerator tries nvidia-smi first, then Apple Silicon unified memory.
func detectAccelerator() hardware {
	if hw, ok := nvidiaGPU(); ok {
		return hw
	}
	if runtime.GOOS == "darwin" {
		name, mem := appleSilicon()
		// Apple unified memory has no separate "free VRAM" or GPU-util we can
		// read cheaply; report total as available and utilization unknown (0).
		return hardware{name: name, vramTotal: mem, vramFree: mem, util: 0}
	}
	return hardware{name: "CPU", vramTotal: 0, vramFree: 0, util: 0}
}

func nvidiaGPU() (hardware, bool) {
	out, err := exec.Command("nvidia-smi",
		"--query-gpu=name,memory.total,memory.free,utilization.gpu",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return hardware{}, false
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	parts := strings.Split(line, ",")
	if len(parts) < 4 {
		return hardware{}, false
	}
	totalMiB, totalErr := strconv.Atoi(strings.TrimSpace(parts[1]))
	freeMiB, freeErr := strconv.Atoi(strings.TrimSpace(parts[2]))
	utilPct, _ := strconv.Atoi(strings.TrimSpace(parts[3]))

	hw := hardware{
		name:      strings.TrimSpace(parts[0]),
		vramTotal: totalMiB / 1024,
		vramFree:  freeMiB / 1024,
		util:      float64(utilPct) / 100,
	}

	// Unified-memory NVIDIA parts (GB10/DGX Spark, Jetson) have no discrete
	// VRAM, so nvidia-smi reports "[N/A]" for memory.total/free. That parses to
	// 0 and would make the scheduler believe the node can't run anything. Treat
	// these like Apple Silicon: fall back to system RAM as the memory pool.
	if totalErr != nil || freeErr != nil || hw.vramTotal == 0 {
		if total, free, ok := linuxSystemMemory(); ok {
			hw.vramTotal = total
			hw.vramFree = free
		}
	}
	return hw, true
}

// linuxSystemMemory reads total and available RAM (in GB) from /proc/meminfo.
// On unified-memory accelerators this is the memory the GPU actually draws from.
func linuxSystemMemory() (total, free int, ok bool) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	var totalKB, availKB int
	for _, ln := range strings.Split(string(data), "\n") {
		fields := strings.Fields(ln)
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.Atoi(fields[1]) // kB
		switch fields[0] {
		case "MemTotal:":
			totalKB = v
		case "MemAvailable:":
			availKB = v
		}
	}
	if totalKB == 0 {
		return 0, 0, false
	}
	return totalKB / (1024 * 1024), availKB / (1024 * 1024), true
}

func appleSilicon() (string, int) {
	chip := "Apple Silicon"
	if out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			chip = s
		}
	}
	mem := 0
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		bytes, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		mem = int(bytes / (1024 * 1024 * 1024))
	}
	return chip, mem
}

func hostName() string {
	// RELAY_NODE_NAME lets an operator override the reported node name. Besides
	// being handy for naming, it makes the multi-node path demoable on a single
	// machine: two `relay join` processes with different names register as two
	// distinct nodes against the same controller.
	if name := strings.TrimSpace(os.Getenv("RELAY_NODE_NAME")); name != "" {
		return name
	}
	if out, err := exec.Command("hostname", "-s").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	return "this-machine"
}

// primaryRuntime picks the runtime the node leads with for display, preferring
// GPU-class engines over the convenience of Ollama when both are present.
func primaryRuntime(detected []string) string {
	pref := []string{"vllm", "llama.cpp", "ollama"}
	for _, p := range pref {
		for _, d := range detected {
			if d == p {
				return d
			}
		}
	}
	return detected[0]
}
