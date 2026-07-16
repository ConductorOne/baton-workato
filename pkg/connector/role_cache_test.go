package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/types/resource"
	"github.com/conductorone/baton-sdk/pkg/types/sessions"
	"github.com/conductorone/baton-workato/pkg/connector/client"
	"github.com/conductorone/baton-workato/pkg/connector/workato"
)

// memSessionStore is a minimal in-memory sessions.SessionStore for tests. It
// honors the WithPrefix option so keys under different prefixes don't collide.
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

// rolesServer returns an httptest server that serves GET /api/roles with the
// given roles on page 0 and an empty page afterward, plus a counter of how many
// times /api/roles was called.
func rolesServer(t *testing.T, roles []*client.Role, calls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/roles" {
			http.NotFound(w, r)
			return
		}
		*calls++
		page := 0
		if raw := r.URL.Query().Get("page"); raw != "" {
			page, _ = strconv.Atoi(raw)
		}
		w.Header().Set("Content-Type", "application/json")
		if page > 0 {
			_ = json.NewEncoder(w).Encode([]*client.Role{})
			return
		}
		_ = json.NewEncoder(w).Encode(roles)
	}))
}

func newTestClient(t *testing.T, baseURL string) *client.WorkatoClient {
	t.Helper()
	c, err := client.NewWorkatoClient(context.Background(), "test-token", baseURL)
	if err != nil {
		t.Fatalf("failed to build client: %v", err)
	}
	return c
}

// TestRoleGrantsSelfHealsEmptyCache is the regression test for the CXP-629
// grant-collapse: when the roles cache is empty for the current sync session
// (e.g. a resumed sync that ran under a new sync id without re-listing roles),
// role.Grants must self-heal by re-listing roles rather than failing with
// NotFound. Before the fix, an empty cache produced a NotFound error that the
// platform skipped, collapsing the grant count and tripping bad-sync.
func TestRoleGrantsSelfHealsEmptyCache(t *testing.T) {
	ctx := context.Background()
	roles := []*client.Role{
		{Id: 175000, Name: "Custom Reviewer", FolderIDs: []int{10}, Privileges: map[string][]string{}},
	}

	calls := 0
	srv := rolesServer(t, roles, &calls)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	b := &roleBuilder{client: c, cache: newCollaboratorCache(c), env: workato.Development}

	res, err := roleResource(roles[0], workato.Development, workato.Development)
	if err != nil {
		t.Fatalf("failed to build role resource: %v", err)
	}

	store := newMemSessionStore() // empty: simulates a session where role.List never populated the cache

	_, _, err = b.Grants(ctx, res, resource.SyncOpAttrs{Session: store})
	if err != nil {
		t.Fatalf("expected self-heal to resolve the role, got error: %v", err)
	}
	if calls == 0 {
		t.Fatalf("expected self-heal to re-list roles, but GetRoles was never called")
	}
	if !rolesCachePopulated(ctx, store) {
		t.Fatalf("expected roles cache to be populated after self-heal")
	}
}

// TestRoleGrantsSkipsGenuinelyAbsentRole verifies that a role which is still
// absent after the self-heal re-list (e.g. deleted between the List and Grants
// phases) is skipped gracefully rather than failing the sync. The self-heal
// runs (GetRoles is called), the role still isn't found, and Grants returns no
// error and no grants.
func TestRoleGrantsSkipsGenuinelyAbsentRole(t *testing.T) {
	ctx := context.Background()
	// The server only knows role 175000; the resource under test is 999999.
	served := []*client.Role{
		{Id: 175000, Name: "Custom Reviewer", FolderIDs: []int{10}, Privileges: map[string][]string{}},
	}

	calls := 0
	srv := rolesServer(t, served, &calls)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	b := &roleBuilder{client: c, cache: newCollaboratorCache(c), env: workato.Development}

	ghost, err := roleResource(&client.Role{Id: 999999, Name: "Ghost"}, workato.Development, workato.Development)
	if err != nil {
		t.Fatalf("failed to build role resource: %v", err)
	}

	store := newMemSessionStore() // empty cache

	grants, _, err := b.Grants(ctx, ghost, resource.SyncOpAttrs{Session: store})
	if err != nil {
		t.Fatalf("expected genuinely-absent role to be skipped, got error: %v", err)
	}
	if calls == 0 {
		t.Fatalf("expected self-heal to attempt a re-list before skipping")
	}
	if len(grants) != 0 {
		t.Fatalf("expected 0 grants for a genuinely-absent role, got %d", len(grants))
	}
}

// TestSelfHealSetRolesCacheIdempotent locks in the fix for the self-heal
// duplicate-grants hazard: the self-heal path (mergeFolderRoles=false) holds the
// complete role set, so running it more than once must not double folder_roles.
// This models a retry after a partial write that set folder_roles but died before
// the populated sentinel (e.g. a transient session-store write failure). Before
// the fix, setRolesCache merged into the existing folder_roles and re-appended the
// same roles, doubling the folder's collaborator-access grants on the next sync.
func TestSelfHealSetRolesCacheIdempotent(t *testing.T) {
	ctx := context.Background()
	roles := []*client.Role{
		{Id: 175000, Name: "Custom Reviewer", FolderIDs: []int{10}, Privileges: map[string][]string{}},
	}
	store := newMemSessionStore()

	if err := setRolesCache(ctx, store, roles, false); err != nil {
		t.Fatalf("first self-heal setRolesCache: %v", err)
	}
	// Second self-heal write (sentinel lost after a partial prior write).
	if err := setRolesCache(ctx, store, roles, false); err != nil {
		t.Fatalf("second self-heal setRolesCache: %v", err)
	}

	folderRoles, err := getRoleByFolder(ctx, store, "10")
	if err != nil {
		t.Fatalf("getRoleByFolder: %v", err)
	}
	if len(folderRoles) != 1 {
		t.Fatalf("folder 10: got %d roles after two self-heals, want 1 (no duplication)", len(folderRoles))
	}
}

// TestFolderGrantsSelfHealsEmptyCache verifies the folder grant path also heals
// an empty roles cache: a folder scoped to a role must still emit its
// collaborator-access grant after self-heal.
func TestFolderGrantsSelfHealsEmptyCache(t *testing.T) {
	ctx := context.Background()
	roles := []*client.Role{
		{Id: 175000, Name: "Custom Reviewer", FolderIDs: []int{10}, Privileges: map[string][]string{}},
	}

	calls := 0
	srv := rolesServer(t, roles, &calls)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	fb := &folderBuilder{client: c, cache: newCollaboratorCache(c)}

	// Folder id 10 is scoped to role 175000 via FolderIDs above.
	parentID := &v2.ResourceId{ResourceType: projectResourceType.Id, Resource: "100"}
	folderRes, err := folderResource(&client.Folder{Id: 10, Name: "Invoices"}, parentID)
	if err != nil {
		t.Fatalf("failed to build folder resource: %v", err)
	}

	store := newMemSessionStore() // empty cache

	grants, _, err := fb.Grants(ctx, folderRes, resource.SyncOpAttrs{Session: store})
	if err != nil {
		t.Fatalf("expected self-heal to resolve folder roles, got error: %v", err)
	}
	if calls == 0 {
		t.Fatalf("expected self-heal to re-list roles for folder grants")
	}
	if len(grants) == 0 {
		t.Fatalf("expected a collaborator-access grant for the folder scoped to role 175000")
	}
}
