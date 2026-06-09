package runtime

import (
	"os/exec"

	"relay/internal/types"
)

// LlamaCPP adapter. llama.cpp shines on Apple Silicon and CPU/edge hosts. Its
// server binary is conventionally `llama-server` on :8080.
type LlamaCPP struct{}

const llamacppAddr = "127.0.0.1:8080"
const llamacppURL = "http://127.0.0.1:8080"

func (l *LlamaCPP) Name() string { return "llama.cpp" }

func (l *LlamaCPP) Detect() bool {
	for _, bin := range []string{"llama-server", "llama-cli", "llama"} {
		if _, err := exec.LookPath(bin); err == nil {
			return true
		}
	}
	return portOpen(llamacppAddr)
}

// DiscoverModels queries llama-server's OpenAI-compatible GET /v1/models. Only
// returns anything when a server is actually running; the binary being on PATH
// is not enough to know which gguf is loaded.
func (l *LlamaCPP) DiscoverModels() ([]types.Model, error) {
	return openAIModels(llamacppURL)
}

func (l *LlamaCPP) Run(job types.Job) error {
	return describe("llama-server -m %s.gguf --port 8080", job.Model)
}

func (l *LlamaCPP) Pull(model types.Model) error {
	return describe("download %s.gguf", model.Name)
}

func (l *LlamaCPP) Stop(jobID string) error {
	return describe("llama-server stop (job %s)", jobID)
}

func (l *LlamaCPP) Health() Status {
	if portOpen(llamacppAddr) {
		return Status{Healthy: true, Detail: "server on " + llamacppAddr}
	}
	return Status{Healthy: false, Detail: "no server on " + llamacppAddr}
}
