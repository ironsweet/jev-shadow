package jev

import (
	"context"
)

// Gate enforces sequential ordering of operations
type Gate struct {
	token chan struct{}
	slots chan struct{}
}

type GateOptions struct {
	// MaxInFlight limits the number of valid tasks entering the gate. A task
	// becomes invalid if its context is canceled.
	MaxInFlight int
}

func NewGate(opts GateOptions) *Gate {
	if opts.MaxInFlight < 1 {
		panic("MaxInFlight must be at least 1")
	}

	g := &Gate{
		token: make(chan struct{}, 1),
		slots: make(chan struct{}, opts.MaxInFlight),
	}
	return g
}

func (g *Gate) Run(ctx context.Context, fn func(context.Context) error) error {
	select {
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
	case <-ctx.Done():
		return ctx.Err()
	case g.token <- struct{}{}:
		defer func() {
			<-g.token
		}()
		return fn(ctx)
	}
}
