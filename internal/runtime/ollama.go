package runtime

import (
	"os"
	"os/exec"

	"relay/internal/types"
)

// Ollama adapter. Ollama is detected by its CLI on PATH or its default API
// port (11434). It exposes a clean HTTP API, so it's the one runtime where
// Phase 0 already does real model and running-job discovery.
type Ollama struct{}

const ollamaURL = "http://127.0.0.1:11434"
const ollamaAddr = "127.0.0.1:11434"

func (o *Ollama) Name() string { return "ollama" }

func (o *Ollama) Detect() bool {
	if _, err := exec.LookPath("ollama"); err == nil {
		return true
	}
	return portOpen(ollamaAddr)
}

// DiscoverModels lists locally pulled models via GET /api/tags.
func (o *Ollama) DiscoverModels() ([]types.Model, error) {
	var body struct {
		Models []struct {
			Name    string `json:"name"`
			Size    int64  `json:"size"`
			Details struct {
				ParameterSize string `json:"parameter_size"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := getJSON(ollamaURL+"/api/tags", &body); err != nil {
		return nil, err
	}
	out := make([]types.Model, 0, len(body.Models))
	for _, m := range body.Models {
		out = append(out, types.Model{
			Name:   m.Name,
			Params: m.Details.ParameterSize,
			Size:   bytesToGB(m.Size),
		})
	}
	return out, nil
}

// Running reports models currently loaded in VRAM via GET /api/ps. Node is
// left blank for the store to fill with the local node's name.
func (o *Ollama) Running() ([]types.Job, error) {
	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := getJSON(ollamaURL+"/api/ps", &body); err != nil {
		return nil, err
	}
	out := make([]types.Job, 0, len(body.Models))
	for _, m := range body.Models {
		out = append(out, types.Job{Model: m.Name, Runtime: o.Name()})
	}
	return out, nil
}

// Run hands the terminal to `ollama run <model>` as an interactive session.
// Ollama auto-pulls the model if it isn't cached, then drops into its own REPL;
// control returns to Relay only when the user exits (Ctrl-D / /bye). Stdio is
// wired to the real TTY so the prompt and streaming output work normally.
func (o *Ollama) Run(job types.Job) error {
	cmd := exec.Command("ollama", "run", job.Model)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (o *Ollama) Pull(model types.Model) error {
	return describe("ollama pull %s", model.Name)
}

func (o *Ollama) Stop(jobID string) error {
	return describe("ollama stop (job %s)", jobID)
}

func (o *Ollama) Health() Status {
	if portOpen(ollamaAddr) {
		return Status{Healthy: true, Detail: "api on " + ollamaAddr}
	}
	return Status{Healthy: false, Detail: "no api on " + ollamaAddr}
}
