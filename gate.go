package jev

import (
	"context"
	"sync"
)

// Gate enforces sequential ordering of operations
type Gate struct {
	token     chan struct{}
	slots     chan struct{}
	done      chan struct{}
	closeOnce sync.Once
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

	g := &Gate{
		token: make(chan struct{}, 1),
		slots: make(chan struct{}, opts.MaxInFlight),
		done:  make(chan struct{}),
	}
	// Auto close the gate when the context is canceled
	go func() {
		select {
		case <-opts.Context.Done():
			g.Close()
		case <-g.done:
		}
	}()
	return g
}

func (g *Gate) Run(ctx context.Context, fn func(context.Context) error) error {
	select {
	case <-g.done:
		return ErrGateClosed
	case g.slots <- struct{}{}:
		// Acquired a slot, proceed
	default:
		return ErrQueueFull
	}
	// Release the slot when done
	defer func() {
		<-g.slots
	}()

	// Ensure sequential execution
	select {
	case <-g.done:
		return ErrGateClosed
	case <-ctx.Done():
		return ctx.Err()
	case g.token <- struct{}{}:
		defer func() {
			<-g.token
		}()
		return fn(ctx)
	}
}

func (g *Gate) Close() {
	g.closeOnce.Do(func() {
		close(g.done)
	})
}
