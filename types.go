package jev

import (
	"errors"
	"time"
)

type Question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
}

type SystemOneRequest struct {
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}

type SystemOneResponse struct {
	Model   string                    `json:"model"`
	Answers map[string]map[string]any `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		StateTokens  int `json:"state_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	LatencyMs float64 `json:"latency_ms"`
	Timing    struct {
		TotalMs float64 `json:"total_ms"`
	} `json:"timing"`
	Backend string `json:"backend"`
}

func (r *SystemOneResponse) NoulProbability(qid string) (float64, bool) {
	if ans, ok := r.Answers[qid]; ok {
		if noul, ok := ans["noul"]; ok {
			if prob, ok := noul.(float64); ok {
				return prob, true
			}
		}
	}
	return 0, false
}

var ErrQueueFull = errors.New("queue full")
var ErrRecorderClosed = errors.New("recorder closed")
var ErrShadowClosed = errors.New("shadow closed")

type Entry struct {
	Timestamp       time.Time `json:"timestamp"`
	UserIntent      string    `json:"user_intent"`
	ToolCalls       string    `json:"tool_calls"`
	ExistingVerdict string    `json:"existing_verdict"`
	JEVProbability  float64   `json:"jev_probability"`
	LatencyMs       float64   `json:"latency_ms"`
}
