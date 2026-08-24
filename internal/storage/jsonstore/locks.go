package jsonstore

import "sync"

type permitLocks struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}

type lockEntry struct {
	mu   sync.Mutex
	refs int
}

func newPermitLocks() *permitLocks { return &permitLocks{locks: map[string]*lockEntry{}} }

func (p *permitLocks) lock(id string) func() {
	p.mu.Lock()
	e := p.locks[id]
	if e == nil {
		e = &lockEntry{}
		p.locks[id] = e
	}
	e.refs++
	p.mu.Unlock()
	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		p.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(p.locks, id)
		}
		p.mu.Unlock()
	}
}
