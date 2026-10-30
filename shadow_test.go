package jev

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeJudger struct {
	stop chan struct{}
}

func (f *fakeJudger) Judge(_, _, _ string) (float64, error) {
	time.Sleep(500 * time.Millisecond) // Simulate some processing time
	f.stop <- struct{}{}
	return 0.88, nil
}

type fakeRecorder struct {
	entries []Entry
}

func (f *fakeRecorder) Record(userIntent, toolCalls, existingVerdict string, jevProbability, jevLatencyMs float64) error {
	f.entries = append(f.entries, Entry{
		Timestamp:       time.Now(),
		UserIntent:      userIntent,
		ToolCalls:       toolCalls,
		ExistingVerdict: existingVerdict,
		JEVProbability:  jevProbability,
		LatencyMs:       jevLatencyMs,
	})
	return nil
}

func (f *fakeRecorder) Close() error {
	return nil
}

func TestShadow_SubmitRecordsSuccess(t *testing.T) {
	done := make(chan struct{})
	judger := &fakeJudger{stop: done}
	recorder := &fakeRecorder{}
	gate := NewGate(GateOptions{MaxInFlight: 1})
	s := NewShadow(judger, recorder, gate)
	start := time.Now()
	assert.NoError(t, s.Submit("intent", "tool calls", "existing verdict", 0.5, 100))
	assert.LessOrEqual(t, time.Since(start), 20*time.Millisecond, "Submit should return quickly, not wait for judger")

	<-done // Wait for the judger to finish
	assert.Equal(t, 1, len(recorder.entries), "Recorder should have one entry")
	entry := recorder.entries[0]
	assert.Equal(t, "intent", entry.UserIntent)
	assert.Equal(t, "tool calls", entry.ToolCalls)
	assert.Equal(t, "existing verdict", entry.ExistingVerdict)
	assert.Equal(t, 0.5, entry.JEVProbability)
	assert.Equal(t, 100.0, entry.LatencyMs)
}
