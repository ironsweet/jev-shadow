package jev

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Judge interface {
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
	judge    Judge
	recorder Recorder
	gate     GateRunner
	logger   Logger

	wg sync.WaitGroup
}

type ShadowOption func(*Shadow)

func WithLogger(logger Logger) ShadowOption {
	return func(s *Shadow) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func NewShadow(judge Judge, recorder Recorder, gate GateRunner, opts ...ShadowOption) *Shadow {
	s := &Shadow{
		judge:    judge,
		recorder: recorder,
		gate:     gate,
		logger:   simpleSlogAdapter(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *Shadow) WithLogger(logger Logger) *Shadow {
	s.logger = logger
	return s
}

func (s *Shadow) Submit(userIntent, toolCalls, existingVerdict string) {
	s.wg.Go(func() {
		defer func() {
			r := recover()
			if r != nil {
				s.logger.Warn(context.Background(), "panic in Submit", "error", r)
			}
		}()

		ctx := context.Background()
		err := s.gate.Run(ctx, func(ctx context.Context) error {
			prob, latency, err := s.judge.Judge(ctx, userIntent, toolCalls)
			if err != nil {
				return fmt.Errorf("judge error: %w", err)
			}

			entry := Entry{
				Timestamp:       time.Now(),
				UserIntent:      userIntent,
				ToolCalls:       toolCalls,
				ExistingVerdict: existingVerdict,
				JEVProbability:  prob,
				LatencyMs:       latency,
			}
			err = s.recorder.Record(entry)
			if err != nil {
				return fmt.Errorf("recorder error: %w", err)
			}
			return nil
		})
		if err != nil {
			s.logger.Warn(ctx, "gate run error", "error", err)
		}
	})
}

func (s *Shadow) Wait() {
	s.wg.Wait()
}
