package jev

import (
	"context"
	"sync"
)

type Task struct {
	ctx context.Context
	run func(context.Context) error
	ret chan error
}

// Gate enforces sequential ordering of operations
type Gate struct {
	sync.Mutex

	waiting chan *Task
	numFree int
	cancel  context.CancelFunc
}

type GateOptions struct {
	MaxInFlight int
	Context     context.Context
}

func NewGate(opts GateOptions) *Gate {
	if opts.Context == nil {
		opts.Context = context.Background()
	}

	waiting := make(chan *Task)

	ctx, cancel := context.WithCancel(opts.Context)
	go func(ctx context.Context) {
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				return
			case task := <-waiting:
				// Check if the task's context is already canceled
				if task.ctx.Err() != nil {
					task.ret <- task.ctx.Err()
					continue
				}
				task.ret <- task.run(task.ctx)
			}
		}
	}(ctx)

	return &Gate{
		waiting: waiting,
		numFree: opts.MaxInFlight,
		cancel:  cancel,
	}
}

func (g *Gate) Run(ctx context.Context, fn func(context.Context) error) error {
	// Fast-fail if the queue is full
	g.Lock()
	if g.numFree == 0 {
		g.Unlock()
		return ErrQueueFull
	}
	g.numFree--
	g.Unlock()

	// Submit the task and wait for the result
	chRet := make(chan error, 1)
	select {
	case g.waiting <- &Task{
		ctx: ctx,
		run: fn,
		ret: chRet,
	}:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Wait for the result or context cancellation
	select {
	case err := <-chRet:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Gate) Close() {
	g.cancel()
}
