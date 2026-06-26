package eventbus

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventCenterConcurrentDifferentEventsRaceFree(t *testing.T) {
	center := NewEventCenter[int]()
	const workers = 32
	const iterations = 1000

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			eventName := fmt.Sprintf("event-%d", worker)
			for i := 0; i < iterations; i++ {
				id, _ := center.AddListener(eventName, func(int) {})
				if err := center.Emit(eventName, strconv.Itoa(i), i); err != nil {
					t.Errorf("emit failed: %v", err)
				}
				_ = center.EventListenerCount(eventName)
				center.DeleteListener(eventName, id)
			}
		}()
	}
	wg.Wait()
}

func TestEventCenterConcurrentSameEventRaceFree(t *testing.T) {
	center := NewEventCenter[int]()
	const workers = 32
	const iterations = 1000
	const eventName = "shared-event"

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				id, _ := center.AddListener(eventName, func(int) {})
				if err := center.Emit(eventName, strconv.Itoa(i), i); err != nil {
					t.Errorf("emit failed: %v", err)
				}
				_ = center.EventListenerCount(eventName)
				center.DeleteListener(eventName, id)
			}
		}()
	}
	wg.Wait()
}

func TestEventCenterEmitUsesHandlerSnapshot(t *testing.T) {
	center := NewEventCenter[int]()
	started := make(chan struct{})
	release := make(chan struct{})
	called := make(chan struct{}, 1)

	id, _ := center.AddListener("event", func(int) {
		close(started)
		<-release
		called <- struct{}{}
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = center.Emit("event", "m1", 1)
	}()

	select {
	case <-started:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("snapshot handler did not start")
	}
	center.DeleteListener("event", id)
	close(release)
	<-done

	select {
	case <-called:
	default:
		select {
		case <-called:
		case <-time.After(300 * time.Millisecond):
			t.Fatal("snapshot handler was not called")
		}
	}
}

func TestEventCenterHandlerCanReenterEventCenter(t *testing.T) {
	center := NewEventCenter[int]()
	done := make(chan struct{})

	var id string
	id, _ = center.AddListener("event", func(int) {
		center.DeleteListener("event", id)
		center.AddListener("event", func(int) {})
		_ = center.Emit("other", "nested", 1)
		close(done)
	})

	if err := center.Emit("event", "m1", 1); err != nil {
		t.Fatalf("emit failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("handler reentry appears to deadlock")
	}
}

func TestEventCenterAddListenerReturnsLatestMeta(t *testing.T) {
	center := NewEventCenter[int]()

	if err := center.Emit("event", "m1", 1); err != nil {
		t.Fatalf("emit failed: %v", err)
	}

	_, meta := center.AddListener("event", func(int) {})
	if meta != "m1" {
		t.Fatalf("expected latest meta m1, got %q", meta)
	}
}

func TestEventCenterDeleteLastListenerKeepsLatestMeta(t *testing.T) {
	center := NewEventCenter[int]()

	id, _ := center.AddListener("event", func(int) {})
	if err := center.Emit("event", "m1", 1); err != nil {
		t.Fatalf("emit failed: %v", err)
	}
	center.DeleteListener("event", id)

	_, meta := center.AddListener("event", func(int) {})
	if meta != "m1" {
		t.Fatalf("expected latest meta m1 after delete, got %q", meta)
	}
}

func TestEventCenterEmitReturnsWithoutWaitingForSlowListener(t *testing.T) {
	center := NewEventCenter[int]()
	started := make(chan struct{})
	release := make(chan struct{})

	center.AddListener("event", func(int) {
		close(started)
		<-release
	})

	done := make(chan struct{})
	go func() {
		_ = center.Emit("event", "m1", 1)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("emit should return quickly even when listener is slow")
	}

	select {
	case <-started:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("listener did not receive emitted event")
	}
	close(release)
}

func TestEventCenterQueueOverflowDropsOldestAndKeepsLatest(t *testing.T) {
	center := NewEventCenter[int](WithListenerQueueCapacity(2))
	releaseFirst := make(chan struct{})
	got := make(chan int, 3)
	var seenFirst atomic.Bool

	center.AddListener("event", func(v int) {
		if seenFirst.CompareAndSwap(false, true) {
			<-releaseFirst
		}
		got <- v
	})

	_ = center.Emit("event", "m0", 0)
	// Let the first callback start and block so subsequent emits fill the queue.
	time.Sleep(10 * time.Millisecond)
	_ = center.Emit("event", "m1", 1)
	_ = center.Emit("event", "m2", 2)
	_ = center.Emit("event", "m3", 3)
	close(releaseFirst)

	want := []int{0, 2, 3}
	for idx, expected := range want {
		select {
		case gotV := <-got:
			if gotV != expected {
				t.Fatalf("got[%d] = %d, want %d", idx, gotV, expected)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting event %d", expected)
		}
	}
}

func TestEventCenterDeleteListenerDropsPendingEvents(t *testing.T) {
	center := NewEventCenter[int](WithListenerQueueCapacity(8))
	release := make(chan struct{})
	got := make(chan int, 8)

	id, _ := center.AddListener("event", func(v int) {
		if v == 0 {
			<-release
		}
		got <- v
	})

	_ = center.Emit("event", "m0", 0)
	time.Sleep(10 * time.Millisecond)
	_ = center.Emit("event", "m1", 1)
	_ = center.Emit("event", "m2", 2)

	center.DeleteListener("event", id)
	close(release)

	select {
	case first := <-got:
		if first != 0 {
			t.Fatalf("first delivered event = %d, want 0", first)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting first callback")
	}

	select {
	case extra := <-got:
		t.Fatalf("unexpected pending event delivered after delete: %d", extra)
	case <-time.After(80 * time.Millisecond):
	}
}
