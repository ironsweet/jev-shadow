package jev

import "context"

type Judger interface {
	Judge(ctx context.Context, userIntent, toolCalls string) (float64, float64, error)
}

type Recorder interface {
	Record(entry Entry) error
	Close() error
}

type GateRunner interface {
	Run(ctx context.Context, fn func(context.Context) error) error
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

func (s *Shadow) Submit(userIntent, toolCalls, existingVerdict string) {
	panic("niy")
}

func (s *Shadow) Wait() error {
	panic("niy")
}

func (s *Shadow) Close() error {
	panic("niy")
}
