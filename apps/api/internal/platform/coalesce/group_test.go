package coalesce

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGroupCoalescesEqualWork(t *testing.T) {
	t.Parallel()

	var group Group[string, int]
	var calls atomic.Int64
	started := make(chan struct{})
	release := make(chan struct{})
	work := func(context.Context) (int, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return 42, nil
	}

	const callers = 16
	results := make(chan int, callers)
	errorsByCaller := make(chan error, callers)
	var waitGroup sync.WaitGroup
	for range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			value, err := group.Do(context.Background(), "same", work)
			results <- value
			errorsByCaller <- err
		}()
	}
	<-started
	waitForWaiters(t, &group, "same", callers)
	close(release)
	waitGroup.Wait()
	close(results)
	close(errorsByCaller)

	if calls.Load() != 1 {
		t.Fatalf("work calls = %d, want 1", calls.Load())
	}
	for err := range errorsByCaller {
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
	}
	for value := range results {
		if value != 42 {
			t.Fatalf("Do() value = %d, want 42", value)
		}
	}
}

func TestGroupKeepsWorkForRemainingWaiter(t *testing.T) {
	t.Parallel()

	var group Group[string, string]
	leaderContext, cancelLeader := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	workContext := make(chan context.Context, 1)
	work := func(ctx context.Context) (string, error) {
		workContext <- ctx
		close(started)
		select {
		case <-release:
			return "complete", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	leaderResult := make(chan error, 1)
	go func() {
		_, err := group.Do(leaderContext, "same", work)
		leaderResult <- err
	}()
	<-started

	followerResult := make(chan struct {
		value string
		err   error
	}, 1)
	go func() {
		value, err := group.Do(context.Background(), "same", work)
		followerResult <- struct {
			value string
			err   error
		}{value: value, err: err}
	}()
	waitForWaiters(t, &group, "same", 2)

	cancelLeader()
	if err := <-leaderResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("leader error = %v", err)
	}
	ctx := <-workContext
	select {
	case <-ctx.Done():
		t.Fatal("shared work was cancelled while a follower remained")
	default:
	}

	close(release)
	follower := <-followerResult
	if follower.err != nil || follower.value != "complete" {
		t.Fatalf("follower result = %+v", follower)
	}
}

func TestGroupCancelsWorkAfterEveryWaiterLeaves(t *testing.T) {
	t.Parallel()

	var group Group[string, int]
	firstContext, cancelFirst := context.WithCancel(context.Background())
	secondContext, cancelSecond := context.WithCancel(context.Background())
	started := make(chan struct{})
	cancelled := make(chan struct{})
	work := func(ctx context.Context) (int, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return 0, ctx.Err()
	}

	results := make(chan error, 2)
	go func() {
		_, err := group.Do(firstContext, "same", work)
		results <- err
	}()
	<-started
	go func() {
		_, err := group.Do(secondContext, "same", work)
		results <- err
	}()
	waitForWaiters(t, &group, "same", 2)

	cancelFirst()
	cancelSecond()
	for range 2 {
		if err := <-results; !errors.Is(err, context.Canceled) {
			t.Fatalf("Do() error = %v", err)
		}
	}
	<-cancelled

	value, err := group.Do(
		context.Background(),
		"same",
		func(context.Context) (int, error) { return 7, nil },
	)
	if err != nil || value != 7 {
		t.Fatalf("replacement Do() = %d, %v", value, err)
	}
}

func waitForWaiters[K comparable, V any](
	t *testing.T,
	group *Group[K, V],
	key K,
	want int,
) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		group.mu.Lock()
		current := group.flights[key]
		waiters := 0
		if current != nil {
			waiters = current.waiters
		}
		group.mu.Unlock()
		if waiters == want {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("waiters did not reach %d", want)
}
