package connector

import (
	"context"
	"sync"
	"testing"

	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-sdk/pkg/types/sessions"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
)

// memSessionStore is a minimal in-memory sessions.SessionStore for tests. It
// honors the WithPrefix option so keys written under one prefix do not collide
// with another, mirroring how the real store scopes cached roles.
type memSessionStore struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newMemSessionStore() *memSessionStore {
	return &memSessionStore{data: make(map[string][]byte)}
}

func (m *memSessionStore) bag(ctx context.Context, opt ...sessions.SessionStoreOption) (*sessions.SessionStoreBag, error) {
	bag := &sessions.SessionStoreBag{}
	for _, o := range opt {
		if err := o(ctx, bag); err != nil {
			return nil, err
		}
	}
	return bag, nil
}

func (m *memSessionStore) storageKey(bag *sessions.SessionStoreBag, key string) string {
	return bag.Prefix + "\x00" + key
}

func (m *memSessionStore) Get(ctx context.Context, key string, opt ...sessions.SessionStoreOption) ([]byte, bool, error) {
	bag, err := m.bag(ctx, opt...)
	if err != nil {
		return nil, false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[m.storageKey(bag, key)]
	return v, ok, nil
}

func (m *memSessionStore) GetMany(ctx context.Context, keys []string, opt ...sessions.SessionStoreOption) (map[string][]byte, []string, error) {
	bag, err := m.bag(ctx, opt...)
	if err != nil {
		return nil, nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	found := make(map[string][]byte)
	var missing []string
	for _, k := range keys {
		if v, ok := m.data[m.storageKey(bag, k)]; ok {
			found[k] = v
		} else {
			missing = append(missing, k)
		}
	}
	return found, missing, nil
}

func (m *memSessionStore) Set(ctx context.Context, key string, value []byte, opt ...sessions.SessionStoreOption) error {
	bag, err := m.bag(ctx, opt...)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[m.storageKey(bag, key)] = value
	return nil
}

func (m *memSessionStore) SetMany(ctx context.Context, values map[string][]byte, opt ...sessions.SessionStoreOption) error {
	bag, err := m.bag(ctx, opt...)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range values {
		m.data[m.storageKey(bag, k)] = v
	}
	return nil
}

func (m *memSessionStore) Delete(ctx context.Context, key string, opt ...sessions.SessionStoreOption) error {
	bag, err := m.bag(ctx, opt...)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, m.storageKey(bag, key))
	return nil
}

func (m *memSessionStore) Clear(_ context.Context, _ ...sessions.SessionStoreOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string][]byte)
	return nil
}

func (m *memSessionStore) GetAll(_ context.Context, _ string, _ ...sessions.SessionStoreOption) (map[string][]byte, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string][]byte, len(m.data))
	for k, v := range m.data {
		out[k] = v
	}
	return out, "", nil
}

// TestRoleGrantsMissingRoleSkips is the repro/regression test for CXP-629's
// follow-up: when a custom role was listed but is absent from the sync session
// cache at grant time (e.g. deleted between the List and Grants phases, or the
// cache is not yet consistent), Grants must skip that role rather than return a
// NotFound error that fails the entire sync. Before the fix this produced
// intermittent "role ... not found" errors and errored_no_data syncs.
func TestRoleGrantsMissingRoleSkips(t *testing.T) {
	ctx := context.Background()
	b := &roleBuilder{
		env:                    workato.Development,
		disableCustomRolesSync: false,
	}

	role := &client.Role{Id: 175962, Name: "Custom Reviewer"}
	res, err := roleResource(role, workato.Development, workato.Development)
	if err != nil {
		t.Fatalf("failed to build role resource: %v", err)
	}

	t.Run("missing from cache -> skip, no error", func(t *testing.T) {
		store := newMemSessionStore() // empty: role never cached

		grants, _, err := b.Grants(ctx, res, resource.SyncOpAttrs{Session: store})
		if err != nil {
			t.Fatalf("expected no error when role missing from cache, got: %v", err)
		}
		if len(grants) != 0 {
			t.Fatalf("expected 0 grants when role skipped, got %d", len(grants))
		}
	})

	t.Run("present in cache -> no error", func(t *testing.T) {
		store := newMemSessionStore()
		if err := setRolesCache(ctx, store, []*client.Role{
			{Id: 175962, Name: "Custom Reviewer", Privileges: map[string][]string{}},
		}); err != nil {
			t.Fatalf("failed to seed roles cache: %v", err)
		}

		_, _, err := b.Grants(ctx, res, resource.SyncOpAttrs{Session: store})
		if err != nil {
			t.Fatalf("expected no error when role present in cache, got: %v", err)
		}
	})
}
