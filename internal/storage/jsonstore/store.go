package jsonstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"confinedpermit/internal/application"
	"confinedpermit/internal/domain"
)

type Store struct {
	path        string
	mu          sync.RWMutex
	doc         *document
	locks       *permitLocks
	replayCache map[string]*domain.PermitBundle
}

func Open(path string) (*Store, error) {
	d, err := loadDocument(path)
	if err != nil {
		return nil, err
	}
	return &Store{path: path, doc: d, locks: newPermitLocks(), replayCache: map[string]*domain.PermitBundle{}}, nil
}

func (s *Store) Create(ctx context.Context, bundle *domain.PermitBundle, key application.IdempotencyKey) (*domain.PermitBundle, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if got, replay, err := s.replayLocked(key); replay || err != nil {
		return got, replay, err
	}
	if _, exists := s.doc.Permits[bundle.Permit.ID]; exists {
		return nil, false, domain.NewConflict("PERMIT_EXISTS", "许可标识已经存在")
	}
	copy, err := cloneBundle(bundle)
	if err != nil {
		return nil, false, err
	}
	result, err := json.Marshal(copy)
	if err != nil {
		return nil, false, err
	}
	index := idempotencyIndex(key)
	next := cloneDocumentWithBundle(s.doc, copy.Permit.ID, copy, index, idempotencyRecord{Digest: key.Digest, Result: result, CreatedAt: time.Now().UTC()})
	if err := saveDocument(s.path, next); err != nil {
		return nil, false, err
	}
	s.doc = next
	out, err := cloneBundle(copy)
	return out, false, err
}

func (s *Store) Mutate(ctx context.Context, id string, expected int64, key application.IdempotencyKey, fn func(*domain.PermitBundle) error) (*domain.PermitBundle, bool, error) {
	unlock := s.locks.lock(id)
	defer unlock()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if got, replay, err := s.replayLocked(key); replay || err != nil {
		return got, replay, err
	}
	current := s.doc.Permits[id]
	if current == nil {
		return nil, false, domain.NewNotFound("许可")
	}
	if current.Permit.Revision != expected {
		return nil, false, domain.NewConflict("REVISION_CONFLICT", fmt.Sprintf("当前 revision 为 %d", current.Permit.Revision))
	}
	work, err := cloneBundle(current)
	if err != nil {
		return nil, false, err
	}
	if err := fn(work); err != nil {
		return nil, false, err
	}
	work.Permit.Revision++
	result, err := json.Marshal(work)
	if err != nil {
		return nil, false, err
	}
	index := idempotencyIndex(key)
	next := cloneDocumentWithBundle(s.doc, id, work, index, idempotencyRecord{Digest: key.Digest, Result: result, CreatedAt: time.Now().UTC()})
	if err := saveDocument(s.path, next); err != nil {
		return nil, false, err
	}
	s.doc = next
	out, err := cloneBundle(work)
	return out, false, err
}

func (s *Store) Get(ctx context.Context, id string) (*domain.PermitBundle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := s.doc.Permits[id]
	if b == nil {
		return nil, domain.NewNotFound("许可")
	}
	return cloneBundle(b)
}

func (s *Store) List(ctx context.Context) ([]*domain.PermitBundle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.PermitBundle, 0, len(s.doc.Permits))
	for _, bundle := range s.doc.Permits {
		copy, err := cloneBundle(bundle)
		if err != nil {
			return nil, err
		}
		out = append(out, copy)
	}
	return out, nil
}

func (s *Store) Flush(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return saveDocument(s.path, s.doc)
}

func (s *Store) replayLocked(key application.IdempotencyKey) (*domain.PermitBundle, bool, error) {
	index := idempotencyIndex(key)
	record, ok := s.doc.Idempotency[index]
	if !ok {
		return nil, false, nil
	}
	if record.Digest != key.Digest {
		return nil, false, domain.NewConflict("REQUEST_ID_REUSED", "request_id 已用于不同载荷")
	}
	if cached := s.replayCache[index]; cached != nil {
		return cached, true, nil
	}
	var b domain.PermitBundle
	if err := json.Unmarshal(record.Result, &b); err != nil {
		return nil, false, fmt.Errorf("读取幂等结果: %w", err)
	}
	s.replayCache[index] = &b
	return &b, true, nil
}

func idempotencyIndex(key application.IdempotencyKey) string {
	return key.Scope + "\x00" + key.Action + "\x00" + key.RequestID
}
