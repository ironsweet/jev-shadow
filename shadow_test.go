package jev

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

type fakeJudge struct {
	fn func(ctx context.Context, userIntent, toolCalls string) (float64, float64, error)
}

func (f *fakeJudge) Judge(ctx context.Context, userIntent, toolCalls string) (float64, float64, error) {
	return f.fn(ctx, userIntent, toolCalls)
}

type fakeRecorder struct {
	entries []Entry
	fn      func(entry Entry) error
}

func (f *fakeRecorder) Record(entry Entry) error {
	if f.fn != nil {
		return f.fn(entry)
	}
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeRecorder) Close() error {
	return nil
}

type fakeGate struct {
	fn func(ctx context.Context, fn func(context.Context) error) error
}

func (f *fakeGate) Run(ctx context.Context, fn func(context.Context) error) error {
	return f.fn(ctx, fn)
}

func TestShadow_SubmitRecordsSuccess(t *testing.T) {
	judge := &fakeJudge{
		fn: func(ctx context.Context, userIntent, toolCalls string) (float64, float64, error) {
			time.Sleep(500 * time.Millisecond) // Simulate some processing time
			return 0.88, 500, nil
		},
	}
	recorder := &fakeRecorder{}
	gate := NewGate(GateOptions{MaxInFlight: 1})
	s := NewShadow(judge, recorder, gate)
	start := time.Now()
	s.Submit("intent", "tool calls", "existing verdict")
	assert.LessOrEqual(t, time.Since(start), 20*time.Millisecond, "Submit should return immediately, not wait for judge")

	s.Wait()
	assert.Equal(t, 1, len(recorder.entries), "Recorder should have one entry")
	entry := recorder.entries[0]
	assert.Equal(t, "intent", entry.UserIntent)
	assert.Equal(t, "tool calls", entry.ToolCalls)
	assert.Equal(t, "existing verdict", entry.ExistingVerdict)
	assert.Equal(t, 0.88, entry.JEVProbability)
	assert.Equal(t, 500.0, entry.LatencyMs)
}

func TestShadow_FailureIsSilent(t *testing.T) {
	for _, tc := range []struct {
		name   string
		judge  func(ctx context.Context, userIntent, toolCalls string) (float64, float64, error)
		runner func(ctx context.Context, fn func(context.Context) error) error
	}{
		{
			name: "judge returns error",
			judge: func(ctx context.Context, userIntent, toolCalls string) (float64, float64, error) {
				return 0, 0, fmt.Errorf("judge error")
			},
			runner: func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		},
		{
			name: "judge panics",
			judge: func(ctx context.Context, userIntent, toolCalls string) (float64, float64, error) {
				panic("judge panic")
			},
			runner: func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		},
		{
			name: "gate queue full",
			runner: func(ctx context.Context, fn func(context.Context) error) error {
				return ErrQueueFull
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &fakeRecorder{}
			gate := &fakeGate{fn: tc.runner}
			s := NewShadow(&fakeJudge{fn: tc.judge}, recorder, gate)
			s.Submit("intent", "tool calls", "existing verdict")
			// Wait a bit to ensure the background goroutine has run
			s.Wait()
			assert.Equal(t, 0, len(recorder.entries), "Recorder should have no entries on judge error")
		})
	}
}

func TestShadow_RecorderFailureIsSilent(t *testing.T) {
	recorder := &fakeRecorder{
		fn: func(entry Entry) error {
			return fmt.Errorf("recorder error")
		},
	}
	gate := NewGate(GateOptions{MaxInFlight: 1})
	s := NewShadow(&fakeJudge{fn: func(ctx context.Context, userIntent, toolCalls string) (float64, float64, error) {
		return 0.88, 100, nil
	}}, recorder, gate)
	s.Submit("intent", "tool calls", "existing verdict")
	s.Wait()
	assert.Equal(t, 0, len(recorder.entries), "Recorder should have no entries on recorder error")

	recorder.fn = nil
	s.Submit("intent2", "tool calls2", "existing verdict2")
	s.Wait()
	assert.Equal(t, 1, len(recorder.entries), "Recorder should have one entry after successful record")
	entry := recorder.entries[0]
	assert.Equal(t, "intent2", entry.UserIntent)
	assert.Equal(t, "tool calls2", entry.ToolCalls)
	assert.Equal(t, "existing verdict2", entry.ExistingVerdict)
	assert.Equal(t, 0.88, entry.JEVProbability)
	assert.Equal(t, 100.0, entry.LatencyMs)
}

func TestShadow_SerializedByGate(t *testing.T) {
	recorder := &fakeRecorder{}
	gate := NewGate(GateOptions{MaxInFlight: 5})
	var maxConcurrent atomic.Int32
	var numInvoked atomic.Int32
	s := NewShadow(&fakeJudge{fn: func(ctx context.Context, userIntent, toolCalls string) (float64, float64, error) {
		maxConcurrent.Add(1)
		assert.Equal(t, 1, int(maxConcurrent.Load()), "Only one judge invocation should be in flight at a time")
		defer maxConcurrent.Add(-1)
		numInvoked.Add(1)
		return 0.88, 100, nil
	}}, recorder, gate)
	for range 5 {
		s.Submit("intent", "tool calls", "existing verdict")
	}
	s.Wait()
	assert.Equal(t, 5, int(numInvoked.Load()), "Judge should have been invoked 5 times")
	assert.Equal(t, 5, len(recorder.entries), "Recorder should have 5 entries")
}

func TestShadow_NoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	var caseIdx atomic.Int32
	judge := &fakeJudge{
		fn: func(ctx context.Context, userIntent, toolCalls string) (float64, float64, error) {
			defer func() {
				caseIdx.Add(1)
			}()
			if caseIdx.Load() < 5 {
				return 0.88, 100, nil
			} else if caseIdx.Load() < 8 {
				return 0, 0, fmt.Errorf("judge error")
			} else if caseIdx.Load() < 10 {
				panic("judge panic")
			}

			return 0.88, 100, nil
		},
	}
	recorder := &fakeRecorder{}
	gate := NewGate(GateOptions{MaxInFlight: 10})
	s := NewShadow(judge, recorder, gate)
	for range 12 {
		s.Submit("intent", "tool calls", "existing verdict")
	}
	s.Wait()
}
