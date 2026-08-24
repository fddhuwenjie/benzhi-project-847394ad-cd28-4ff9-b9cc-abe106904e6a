package jsonstore

import (
	"context"
	"sync"
)

type permitLocks struct {
	mu    sync.Mutex
	locks map[string]*lockEntry
}

type lockEntry struct {
	token chan struct{}
	refs  int
}

func newPermitLocks() *permitLocks { return &permitLocks{locks: map[string]*lockEntry{}} }

func (p *permitLocks) lock(ctx context.Context, id string) (func(), error) {
	p.mu.Lock()
	e := p.locks[id]
	if e == nil {
		e = &lockEntry{token: make(chan struct{}, 1)}
		e.token <- struct{}{}
		p.locks[id] = e
	}
	e.refs++
	p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		p.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(p.locks, id)
		}
		p.mu.Unlock()
		return nil, err
	}
	<-e.token
	return func() {
		e.token <- struct{}{}
		p.mu.Lock()
		e.refs--
		if e.refs == 0 {
			delete(p.locks, id)
		}
		p.mu.Unlock()
	}, nil
}
