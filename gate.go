package jev

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

type Task struct {
	ctx context.Context
	run func(context.Context) error
	ret chan error
}

// Gate enforces sequential ordering of operations
type Gate struct {
	lock    sync.Mutex
	waiting chan *Task
	numFree int

	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
	closed    atomic.Bool
}

type GateOptions struct {
	// MaxInFlight limits the number of valid tasks entering the gate. A task
	// becomes invalid if its context is canceled.
	MaxInFlight int
	Context     context.Context
}

func NewGate(opts GateOptions) *Gate {
	if opts.MaxInFlight < 1 {
		panic("MaxInFlight must be at least 1")
	}
	if opts.Context == nil {
		opts.Context = context.Background()
	}

	waiting := make(chan *Task)
	ctx, cancel := context.WithCancel(opts.Context)
	g := &Gate{
		waiting: waiting,
		numFree: opts.MaxInFlight,
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case task := <-waiting:
				// Check if the task's context is already canceled
				if task.ctx.Err() != nil {
					continue
				}
				if g.closed.Load() {
					task.ret <- ErrGateClosed
					continue
				}
				func() {
					defer func() {
						r := recover()
						if r != nil {
							task.ret <- &PanicError{Value: r}
						}
					}()

					task.ret <- task.run(task.ctx)
				}()
			}
		}
	}(ctx)

	return g
}

func (g *Gate) Run(ctx context.Context, fn func(context.Context) error) error {
	if g.closed.Load() {
		return ErrGateClosed
	}

	// Fast-fail if the queue is full
	g.lock.Lock()
	if g.numFree == 0 {
		g.lock.Unlock()
		return ErrQueueFull
	}
	g.numFree--
	g.lock.Unlock()

	// Submit the task and wait for the result
	chRet := make(chan error, 1)
	select {
	case g.waiting <- &Task{
		ctx: ctx,
		run: fn,
		ret: chRet,
	}:
	case <-ctx.Done():
		g.release()
		return ctx.Err()
	case <-g.done:
		g.release()
		return ErrGateClosed
	}

	defer g.release()

	// Wait for the result or context cancellation
	select {
	case err := <-chRet:
		var e *PanicError
		if errors.As(err, &e) {
			panic(e.Value)
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-g.done:
		return ErrGateClosed
	}
}

func (g *Gate) release() {
	g.lock.Lock()
	g.numFree++
	g.lock.Unlock()
}

func (g *Gate) Close() {
	g.closeOnce.Do(func() {
		g.closed.Store(true)
		g.cancel()
		close(g.done)
	})
}
