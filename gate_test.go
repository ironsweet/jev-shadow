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
	g := NewDefaultGate()
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

	waitGroup.Go(func() {
		assert.NoError(t, g.Run(t.Context(), func(ctx context.Context) error {
			concurrency.Add(1)
			assert.Equal(t, int32(1), concurrency.Load())
			concurrency.Add(-1)
			return nil
		}))
	})

	time.Sleep(time.Millisecond)
	close(release)
	waitGroup.Wait()
}

func TestGate_RunRespectsContext(t *testing.T) {
	g := NewDefaultGate()
	defer g.Close()

	var waitGroup sync.WaitGroup
	var blockCh = make(chan struct{})

	waitGroup.Go(func() {
		assert.NoError(t, g.Run(t.Context(), func(ctx context.Context) error {
			<-blockCh
			return nil
		}))
	})

	var barrier sync.WaitGroup
	barrier.Go(func() {
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		err := g.Run(ctx, func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		})
		assert.Equal(t, context.DeadlineExceeded, err)
	})
	barrier.Wait()

	blockCh <- struct{}{}
	waitGroup.Wait()
}

func TestGate_RunRejectsWhenQueueFull(t *testing.T) {
	g := NewGate(GateOptions{MaxInFlight: 1, Context: t.Context()})
	defer g.Close()

	var blockCh = make(chan struct{})
	var waitGroup sync.WaitGroup

	waitGroup.Go(func() {
		assert.NoError(t, g.Run(t.Context(), func(ctx context.Context) error {
			<-blockCh
			return nil
		}))
	})

	var barrier sync.WaitGroup
	barrier.Go(func() {
		time.Sleep(time.Millisecond)
		err := g.Run(t.Context(), func(ctx context.Context) error {
			return nil
		})
		assert.Equal(t, ErrQueueFull, err)
	})
	print("Waiting for barrier to finish")
	barrier.Wait()

	blockCh <- struct{}{}
	waitGroup.Wait()
}

func TestGate_RunNoGoroutineLeak(t *testing.T) {
	defer goleak.VerifyNone(t)

	g := NewGate(GateOptions{MaxInFlight: 7, Context: t.Context()})
	defer g.Close()

	var waitGroup sync.WaitGroup
	var concurrency atomic.Int32

	for range 10 {
		waitGroup.Go(func() {
			_ = g.Run(t.Context(), func(ctx context.Context) error {
				concurrency.Add(1)
				assert.Equal(t, int32(1), concurrency.Load())
				concurrency.Add(-1)
				return nil
			})
		})
	}

	for range 10 {
		waitGroup.Go(func() {
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			_ = g.Run(ctx, func(ctx context.Context) error {
				concurrency.Add(1)
				assert.Equal(t, int32(1), concurrency.Load())
				concurrency.Add(-1)
				return nil
			})
		})
	}

	waitGroup.Wait()
}
