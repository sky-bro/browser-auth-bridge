package pool

import (
	"sync/atomic"
	"testing"
	"time"
)

type testPooledWorker struct {
	id int32
}

func TestWorkerPoolScalesReusesAndExpires(t *testing.T) {
	var created, discarded atomic.Int32
	pool := New(
		20*time.Millisecond,
		func() *testPooledWorker { return &testPooledWorker{id: created.Add(1)} },
		func(*testPooledWorker) bool { return true },
		func(*testPooledWorker) { discarded.Add(1) },
	)
	defer pool.Close()

	first := pool.Acquire()
	second := pool.Acquire()
	if first == second || created.Load() != 2 {
		t.Fatalf("busy worker was reused: first=%p second=%p created=%d", first, second, created.Load())
	}
	pool.Release(first)
	if reused := pool.Acquire(); reused != first {
		t.Fatalf("idle worker was not reused: got=%p want=%p", reused, first)
	} else {
		pool.Release(reused)
	}
	pool.Release(second)

	deadline := time.Now().Add(time.Second)
	for discarded.Load() != 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if discarded.Load() != 2 || !pool.Empty() {
		t.Fatalf("idle workers were not reclaimed: discarded=%d Empty=%v", discarded.Load(), pool.Empty())
	}
}

func TestCloseDiscardsActiveOnReleaseAndRejectsNewLease(t *testing.T) {
	var discarded atomic.Int32
	p := New(time.Hour, func() int { return 1 }, func(int) bool { return true }, func(int) { discarded.Add(1) })
	w := p.Acquire()
	p.Close()
	p.Release(w)
	if discarded.Load() != 1 || !p.Empty() {
		t.Fatal("active lease leaked after close")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("acquire after close was accepted")
		}
	}()
	p.Acquire()
}
func TestUnhealthyWorkerIsDiscarded(t *testing.T) {
	var discarded atomic.Int32
	p := New(time.Hour, func() int { return 1 }, func(int) bool { return false }, func(int) { discarded.Add(1) })
	p.Release(p.Acquire())
	p.Close()
	if discarded.Load() != 1 {
		t.Fatal("unhealthy worker leaked")
	}
}
