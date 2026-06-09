package runtime

import (
	"os/exec"

	"relay/internal/types"
)

// VLLM adapter. vLLM targets NVIDIA GPUs and exposes an OpenAI-compatible
// server, conventionally on :8000. Detected by the `vllm` CLI on PATH.
type VLLM struct{}

const vllmAddr = "127.0.0.1:8000"
const vllmURL = "http://127.0.0.1:8000"

func (v *VLLM) Name() string { return "vllm" }

func (v *VLLM) Detect() bool {
	if _, err := exec.LookPath("vllm"); err == nil {
		return true
	}
	return portOpen(vllmAddr)
}

// DiscoverModels queries the OpenAI-compatible GET /v1/models. vLLM doesn't
// report size/params there, so those fields stay zero until we probe deeper.
func (v *VLLM) DiscoverModels() ([]types.Model, error) {
	return openAIModels(vllmURL)
}

func (v *VLLM) Run(job types.Job) error {
	return describe("vllm serve %s --port 8000", job.Model)
}

func (v *VLLM) Pull(model types.Model) error {
	// vLLM pulls from HuggingFace on first serve; an explicit warm-up download
	// would go here.
	return describe("huggingface-cli download %s", model.Name)
}

func (v *VLLM) Stop(jobID string) error {
	return describe("vllm stop (job %s)", jobID)
}

func (v *VLLM) Health() Status {
	if portOpen(vllmAddr) {
		return Status{Healthy: true, Detail: "openai api on " + vllmAddr}
	}
	return Status{Healthy: false, Detail: "no api on " + vllmAddr}
}
