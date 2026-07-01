package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	secutils "github.com/Tencent/XinWiki/internal/utils"
	"gorm.io/gorm"
)

// fakeAPIKeyRepo is an in-memory stand-in for interfaces.APIKeyRepository.
// It embeds the interface so the unused GetByHash/TouchLastUsed methods
// satisfy the compiler without being implemented (these service tests never
// call them; the auth path exercises GetByHash via repo tests).
type fakeAPIKeyRepo struct {
	interfaces.APIKeyRepository // nil; GetByHash/TouchLastUsed panic if called
	rows                        []*types.APIKey
	createErr                   error
}

func (r *fakeAPIKeyRepo) Create(_ context.Context, key *types.APIKey) error {
	if r.createErr != nil {
		return r.createErr
	}
	cp := *key
	r.rows = append(r.rows, &cp)
	return nil
}

func (r *fakeAPIKeyRepo) GetByID(_ context.Context, id string) (*types.APIKey, error) {
	for _, k := range r.rows {
		if k.ID == id {
			cp := *k
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeAPIKeyRepo) ListByTenant(_ context.Context, tenantID uint64) ([]*types.APIKey, error) {
	var out []*types.APIKey
	for _, k := range r.rows {
		if k.TenantID == tenantID {
			cp := *k
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeAPIKeyRepo) Revoke(_ context.Context, id string) error {
	for _, k := range r.rows {
		if k.ID == id {
			k.Status = "revoked"
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

// TestAPIKeyService_Create_HashesSecretAndReturnsPlaintextOnce: the returned
// plaintext secret hashes to the persisted KeyHash, the plaintext is never
// stored, and the row is persisted with status active.
func TestAPIKeyService_Create_HashesSecretAndReturnsPlaintextOnce(t *testing.T) {
	repo := &fakeAPIKeyRepo{}
	svc := NewAPIKeyService(repo)

	key, secret, err := svc.Create(context.Background(), 7, "u1", "CI ingest", []string{"kb:read"}, nil)
	requireNoError(t, err)
	if secret == "" {
		t.Fatalf("plaintext secret must be returned once")
	}
	if len(secret) < 24 {
		t.Errorf("secret entropy too low: %q", secret)
	}
	if key == nil {
		t.Fatalf("key must be returned")
	}
	// The persisted hash must equal HashAPIKey(secret) but NOT equal secret.
	if key.KeyHash != secutils.HashAPIKey(secret) {
		t.Errorf("KeyHash must equal HashAPIKey(secret)")
	}
	if key.KeyHash == secret {
		t.Errorf("plaintext secret must never be stored as KeyHash")
	}
	if key.Status != "active" {
		t.Errorf("new key status = %q, want active", key.Status)
	}
	if key.ID == "" {
		t.Errorf("key ID must be generated")
	}
	if len(repo.rows) != 1 || repo.rows[0].KeyHash != key.KeyHash {
		t.Errorf("key must be persisted exactly once with the hashed secret")
	}
}

// TestAPIKeyService_Create_RejectsEmptyName: a blank name is rejected before
// the repo is touched.
func TestAPIKeyService_Create_RejectsEmptyName(t *testing.T) {
	repo := &fakeAPIKeyRepo{}
	svc := NewAPIKeyService(repo)

	_, _, err := svc.Create(context.Background(), 7, "u1", "   ", []string{"kb:read"}, nil)
	if !errors.Is(err, ErrAPIKeyNameRequired) {
		t.Errorf("empty name want ErrAPIKeyNameRequired, got %v", err)
	}
	if len(repo.rows) != 0 {
		t.Errorf("repo must not be touched on validation failure")
	}
}

// TestAPIKeyService_Create_RejectsInvalidScope: an unknown scope is rejected
// with the types.ErrInvalidAPIKeyScope sentinel (wrapped).
func TestAPIKeyService_Create_RejectsInvalidScope(t *testing.T) {
	repo := &fakeAPIKeyRepo{}
	svc := NewAPIKeyService(repo)

	_, _, err := svc.Create(context.Background(), 7, "u1", "CI", []string{"foo:bar"}, nil)
	if !errors.Is(err, types.ErrInvalidAPIKeyScope) {
		t.Errorf("invalid scope want types.ErrInvalidAPIKeyScope, got %v", err)
	}
	if len(repo.rows) != 0 {
		t.Errorf("repo must not be touched on validation failure")
	}
}

// TestAPIKeyService_Create_PersistsScopesAndExpiry: scopes and expiry are
// carried onto the persisted row verbatim.
func TestAPIKeyService_Create_PersistsScopesAndExpiry(t *testing.T) {
	repo := &fakeAPIKeyRepo{}
	svc := NewAPIKeyService(repo)
	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	key, _, err := svc.Create(context.Background(), 7, "u1", "CI", []string{"kb:read", "doc:*"}, &exp)
	requireNoError(t, err)
	if got := []string(key.Scopes); len(got) != 2 || got[0] != "kb:read" || got[1] != "doc:*" {
		t.Errorf("scopes not persisted: %v", got)
	}
	if key.ExpiresAt == nil || !key.ExpiresAt.Equal(exp) {
		t.Errorf("expires_at not persisted: %v", key.ExpiresAt)
	}
	if key.TenantID != 7 || key.UserID != "u1" || key.Name != "CI" {
		t.Errorf("tenant/user/name not stamped: %+v", key)
	}
}

// TestAPIKeyService_Create_RepoErrorPropagates: a repo failure surfaces as the
// return error with an empty secret (never partially leak the secret).
func TestAPIKeyService_Create_RepoErrorPropagates(t *testing.T) {
	boom := errors.New("db down")
	repo := &fakeAPIKeyRepo{createErr: boom}
	svc := NewAPIKeyService(repo)

	_, secret, err := svc.Create(context.Background(), 7, "u1", "CI", []string{"kb:read"}, nil)
	if !errors.Is(err, boom) {
		t.Errorf("repo error must propagate, got %v", err)
	}
	if secret != "" {
		t.Errorf("secret must be empty on failure, got %q", secret)
	}
}

// TestAPIKeyService_List_ReturnsOnlyTenantKeys: List filters by tenant_id so
// tenant A cannot enumerate tenant B's keys.
func TestAPIKeyService_List_ReturnsOnlyTenantKeys(t *testing.T) {
	repo := &fakeAPIKeyRepo{rows: []*types.APIKey{
		{ID: "ak_a", TenantID: 1, Name: "A1"},
		{ID: "ak_b", TenantID: 2, Name: "B1"},
		{ID: "ak_c", TenantID: 1, Name: "A2"},
	}}
	svc := NewAPIKeyService(repo)

	got, err := svc.List(context.Background(), 1)
	requireNoError(t, err)
	if len(got) != 2 {
		t.Fatalf("tenant 1 should have 2 keys, got %d", len(got))
	}
	for _, k := range got {
		if k.TenantID != 1 {
			t.Errorf("List leaked cross-tenant key: %+v", k)
		}
	}
}

// TestAPIKeyService_Revoke_Success: revoking an owned key flips status to
// revoked.
func TestAPIKeyService_Revoke_Success(t *testing.T) {
	repo := &fakeAPIKeyRepo{rows: []*types.APIKey{
		{ID: "ak_a", TenantID: 1, Status: "active"},
	}}
	svc := NewAPIKeyService(repo)

	requireNoError(t, svc.Revoke(context.Background(), 1, "ak_a"))
	if repo.rows[0].Status != "revoked" {
		t.Errorf("status = %q, want revoked", repo.rows[0].Status)
	}
}

// TestAPIKeyService_Revoke_NotFound: revoking a missing id returns
// ErrAPIKeyNotFound.
func TestAPIKeyService_Revoke_NotFound(t *testing.T) {
	repo := &fakeAPIKeyRepo{}
	svc := NewAPIKeyService(repo)

	err := svc.Revoke(context.Background(), 1, "nope")
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Errorf("missing key want ErrAPIKeyNotFound, got %v", err)
	}
}

// TestAPIKeyService_Revoke_CrossTenantDenied: revoking a key that exists but
// belongs to another tenant returns ErrAPIKeyNotFound (no existence leak)
// and does NOT mutate the row.
func TestAPIKeyService_Revoke_CrossTenantDenied(t *testing.T) {
	repo := &fakeAPIKeyRepo{rows: []*types.APIKey{
		{ID: "ak_b", TenantID: 2, Status: "active"},
	}}
	svc := NewAPIKeyService(repo)

	err := svc.Revoke(context.Background(), 1, "ak_b")
	if !errors.Is(err, ErrAPIKeyNotFound) {
		t.Errorf("cross-tenant revoke want ErrAPIKeyNotFound, got %v", err)
	}
	if repo.rows[0].Status != "active" {
		t.Errorf("cross-tenant revoke must not mutate the row, status = %q", repo.rows[0].Status)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
