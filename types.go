package jev

import (
	"errors"
	"fmt"
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
var ErrGateClosed = errors.New("gate closed")

type PanicError struct {
	Value any
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("panic: %v\n\n%s", e.Value, string(e.Stack))
}

func (e *PanicError) Unwrap() error {
	if err, ok := e.Value.(error); ok {
		return err
	}
	return nil
}

func RecoverPanic(r any) (value any, stack []byte) {
	e, ok := r.(*PanicError)
	if ok {
		return e.Value, e.Stack
	}
	return r, nil
}
