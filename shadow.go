package jev

import "context"

type Judger interface {
	Judge(userIntent, toolCalls, existingVerdict string) (float64, error)
}

type Recorder interface {
	Record(userIntent, toolCalls, existingVerdict string, jevProbability, jevLatencyMs float64) error
	Close() error
}

type GateRunner interface {
	Run(ctx context.Context, fn func(context.Context) error) error
	Close()
}

type Shadow struct {
	judger   Judger
	recorder Recorder
	gate     GateRunner
}

func NewShadow(judger Judger, recorder Recorder, gate GateRunner) *Shadow {
	return &Shadow{
		judger:   judger,
		recorder: recorder,
		gate:     gate,
	}
}

func (s *Shadow) Submit(ctx context.Context, userIntent, toolCalls, existingVerdict string, jevProbability, jevLatencyMs float64) error {
	panic("niy")
}

func (s *Shadow) Wait() error {
	panic("niy")
}

func (s *Shadow) Close() error {
	panic("niy")
}
