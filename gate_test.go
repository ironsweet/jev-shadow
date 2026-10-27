package jev

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestGate_RunSerialize(t *testing.T) {
	g := NewGate(GateOptions{MaxInFlight: 2, Context: t.Context()})
	defer g.Close()

	var concurrency atomic.Int32
	var waitGroup sync.WaitGroup
	firstIn := make(chan struct{})
	release := make(chan struct{})

	waitGroup.Go(func() {
		assert.NoError(t, g.Run(t.Context(), func(ctx context.Context) error {
			concurrency.Add(1)
			assert.Equal(t, int32(1), concurrency.Load())
			close(firstIn)
			<-release
			concurrency.Add(-1)
			return nil
		}))
	})
	<-firstIn // wait for the first task to start

	secondReady := make(chan struct{})
	waitGroup.Go(func() {
		close(secondReady)
		assert.NoError(t, g.Run(t.Context(), func(ctx context.Context) error {
			concurrency.Add(1)
			assert.Equal(t, int32(1), concurrency.Load())
			concurrency.Add(-1)
			return nil
		}))
	})

	<-secondReady // wait for the second task to be ready
	time.Sleep(10 * time.Millisecond)
	close(release)
	waitGroup.Wait()
}

func TestGate_RunRespectsContext(t *testing.T) {
	g := NewGate(GateOptions{MaxInFlight: 2, Context: t.Context()})
	defer g.Close()

	var waitGroup sync.WaitGroup
	firstIn := make(chan struct{})
	release := make(chan struct{})

	waitGroup.Go(func() {
		assert.NoError(t, g.Run(t.Context(), func(ctx context.Context) error {
			close(firstIn)
			<-release
			return nil
		}))
	})
	<-firstIn // wait for the first task to start

	var barrier sync.WaitGroup
	barrier.Go(func() {
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		var fnCalled atomic.Bool
		err := g.Run(ctx, func(ctx context.Context) error {
			fnCalled.Store(true)
			return nil
		})
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.False(t, fnCalled.Load(), "function should not have been called")
	})
	barrier.Wait()

	close(release)
	waitGroup.Wait()
}

func TestGate_RunRejectsWhenQueueFull(t *testing.T) {
	g := NewGate(GateOptions{MaxInFlight: 1, Context: t.Context()})
	defer g.Close()

	var waitGroup sync.WaitGroup
	firstIn := make(chan struct{})
	release := make(chan struct{})

	waitGroup.Go(func() {
		assert.NoError(t, g.Run(t.Context(), func(ctx context.Context) error {
			close(firstIn)
			<-release
			return nil
		}))
	})
	<-firstIn // wait for the first task to start

	var barrier sync.WaitGroup
	barrier.Go(func() {
		var fnCalled atomic.Bool
		err := g.Run(t.Context(), func(ctx context.Context) error {
			fnCalled.Store(true)
			return nil
		})
		assert.ErrorIs(t, err, ErrQueueFull)
		assert.False(t, fnCalled.Load(), "function should not have been called")
	})
	barrier.Wait()

	close(release)
	waitGroup.Wait()
}

func TestGate_RunNoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	g := NewGate(GateOptions{MaxInFlight: 7, Context: t.Context()})
	defer g.Close()

	var waitGroup sync.WaitGroup
	firstIn := make(chan struct{})
	release := make(chan struct{})

	waitGroup.Go(func() {
		assert.NoError(t, g.Run(t.Context(), func(ctx context.Context) error {
			close(firstIn)
			<-release
			return nil
		}))
	})
	<-firstIn // wait for the first task to start

	for range 10 {
		waitGroup.Go(func() {
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			_ = g.Run(ctx, func(ctx context.Context) error {
				return nil
			})
		})
	}

	time.Sleep(10 * time.Millisecond)
	close(release)
	waitGroup.Wait()
}
