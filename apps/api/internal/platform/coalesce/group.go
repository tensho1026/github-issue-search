package coalesce

import (
	"context"
	"sync"
)

type flight[V any] struct {
	cancel  context.CancelFunc
	done    chan struct{}
	err     error
	value   V
	waiters int
}

// Group coalesces concurrent work for equal comparable keys. A caller that
// leaves stops waiting without cancelling work needed by another caller. The
// shared context is cancelled as soon as every waiter has left.
type Group[K comparable, V any] struct {
	mu      sync.Mutex
	flights map[K]*flight[V]
}

// Do joins or starts work for key. The function receives a context that keeps
// the first caller's values but has an independent cancellation lifecycle.
func (group *Group[K, V]) Do(
	ctx context.Context,
	key K,
	work func(context.Context) (V, error),
) (V, error) {
	var zero V
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	group.mu.Lock()
	current, exists := group.flights[key]
	if exists {
		current.waiters++
		group.mu.Unlock()
		return group.wait(ctx, key, current)
	}

	operationContext, cancel := context.WithCancel(context.WithoutCancel(ctx))
	current = &flight[V]{
		cancel:  cancel,
		done:    make(chan struct{}),
		waiters: 1,
	}
	if group.flights == nil {
		group.flights = make(map[K]*flight[V])
	}
	group.flights[key] = current
	group.mu.Unlock()

	go func() {
		defer cancel()
		group.run(operationContext, key, current, work)
	}()
	return group.wait(ctx, key, current)
}

func (group *Group[K, V]) run(
	ctx context.Context,
	key K,
	current *flight[V],
	work func(context.Context) (V, error),
) {
	value, err := work(ctx)

	group.mu.Lock()
	current.value = value
	current.err = err
	if group.flights[key] == current {
		delete(group.flights, key)
	}
	close(current.done)
	group.mu.Unlock()
}

func (group *Group[K, V]) wait(
	ctx context.Context,
	key K,
	current *flight[V],
) (V, error) {
	select {
	case <-current.done:
		return current.value, current.err
	case <-ctx.Done():
		select {
		case <-current.done:
			return current.value, current.err
		default:
		}
		group.leave(key, current)
		var zero V
		return zero, ctx.Err()
	}
}

func (group *Group[K, V]) leave(key K, current *flight[V]) {
	group.mu.Lock()
	defer group.mu.Unlock()

	current.waiters--
	if current.waiters > 0 {
		return
	}
	if group.flights[key] == current {
		delete(group.flights, key)
	}
	current.cancel()
}
