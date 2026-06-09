package runtime

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"relay/internal/types"
)

// probeClient is shared by adapters for talking to local engine APIs. Timeouts
// are short: discovery must never hang the CLI on a wedged server.
var probeClient = &http.Client{Timeout: 400 * time.Millisecond}

// RunningLister is an optional capability: adapters that can report which
// models are live implement it, and the store type-asserts for it. Keeping it
// out of the core Runtime interface avoids forcing every engine to support it.
type RunningLister interface {
	Running() ([]types.Job, error)
}

// portOpen reports whether something is listening at addr.
func portOpen(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// getJSON does a best-effort GET and decodes JSON into out. A failure (engine
// down, bad response) is returned as an error so callers can degrade quietly.
func getJSON(url string, out any) error {
	resp, err := probeClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// openAIModels lists models from an OpenAI-compatible GET /v1/models endpoint,
// shared by the vLLM and llama.cpp adapters. Size/params aren't exposed there,
// so only the model id is populated.
func openAIModels(baseURL string) ([]types.Model, error) {
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := getJSON(baseURL+"/v1/models", &body); err != nil {
		return nil, err
	}
	out := make([]types.Model, 0, len(body.Data))
	for _, m := range body.Data {
		out = append(out, types.Model{Name: m.ID})
	}
	return out, nil
}

// bytesToGB converts a byte count to whole GB, rounding to nearest.
func bytesToGB(b int64) int {
	return int((b + (1 << 29)) >> 30) // +0.5GB then /1GiB
}

// describe is the Phase-0 stand-in for execution: it records the command the
// adapter would run. Replaced by real exec/HTTP calls in a later phase.
func describe(format string, args ...any) error {
	fmt.Printf("    would exec: "+format+"\n", args...)
	return nil
}
