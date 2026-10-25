package pool

import (
	"sync"
	"time"
)

type idleWorker[T any] struct {
	worker T
	timer  *time.Timer
}

// Pool has no size cap: an idle worker is reused, otherwise Acquire
// creates a new one. Each worker is exclusively leased until Release.
type Pool[T any] struct {
	mu          sync.Mutex
	idleTimeout time.Duration
	create      func() T
	reusable    func(T) bool
	discard     func(T)
	idle        []*idleWorker[T]
	active      int
	closed      bool
}

func New[T any](idleTimeout time.Duration, create func() T, reusable func(T) bool, discard func(T)) *Pool[T] {
	return &Pool[T]{idleTimeout: idleTimeout, create: create, reusable: reusable, discard: discard}
}

func (p *Pool[T]) Acquire() T {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		panic("acquire after pool close")
	}
	p.active++
	if last := len(p.idle) - 1; last >= 0 {
		slot := p.idle[last]
		p.idle = p.idle[:last]
		slot.timer.Stop()
		return slot.worker
	}
	return p.create()
}

func (p *Pool[T]) Release(worker T) {
	p.mu.Lock()
	p.active--
	if p.closed || !p.reusable(worker) {
		p.mu.Unlock()
		p.discard(worker)
		return
	}
	slot := &idleWorker[T]{worker: worker}
	slot.timer = time.AfterFunc(p.idleTimeout, func() { p.expire(slot) })
	p.idle = append(p.idle, slot)
	p.mu.Unlock()
}

func (p *Pool[T]) expire(slot *idleWorker[T]) {
	p.mu.Lock()
	found := false
	for i, candidate := range p.idle {
		if candidate == slot {
			p.idle = append(p.idle[:i], p.idle[i+1:]...)
			found = true
			break
		}
	}
	p.mu.Unlock()
	if found {
		p.discard(slot.worker)
	}
}

func (p *Pool[T]) Empty() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active == 0 && len(p.idle) == 0
}

func (p *Pool[T]) Close() {
	p.mu.Lock()
	p.closed = true
	idle := p.idle
	p.idle = nil
	p.mu.Unlock()
	for _, slot := range idle {
		slot.timer.Stop()
		p.discard(slot.worker)
	}
}
