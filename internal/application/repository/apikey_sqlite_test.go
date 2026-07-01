package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// apiKeysTestDDL is a SQLite-compatible subset of the production api_keys DDL
// (migrations/versioned/000066_api_keys.up.sql). StringArray serializes scopes
// as JSON TEXT; the column type is TEXT in SQLite.
const apiKeysTestDDL = `
CREATE TABLE IF NOT EXISTS api_keys (
    id           TEXT PRIMARY KEY,
    tenant_id    INTEGER NOT NULL,
    user_id      TEXT,
    name         TEXT NOT NULL DEFAULT '',
    key_hash     TEXT NOT NULL,
    prefix       TEXT NOT NULL DEFAULT '',
    scopes       TEXT NOT NULL DEFAULT '[]',
    status       TEXT NOT NULL DEFAULT 'active',
    expires_at   DATETIME,
    last_used_at DATETIME,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at   DATETIME
);
`

func newAPIKeyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS api_keys").Error)
	require.NoError(t, db.Exec(apiKeysTestDDL).Error)
	return db
}

func TestAPIKeyRepo_CreateAndGetByHash(t *testing.T) {
	db := newAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	ctx := context.Background()

	k := &types.APIKey{
		ID:       "ak_1",
		TenantID: 7,
		Name:     "CI ingest",
		KeyHash:  "hash-of-sk_abc",
		Prefix:   "sk_abc",
		Scopes:   types.StringArray{"kb:read", "doc:write"},
		Status:   "active",
	}
	require.NoError(t, repo.Create(ctx, k))

	got, err := repo.GetByHash(ctx, "hash-of-sk_abc")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "ak_1", got.ID)
	require.Equal(t, uint64(7), got.TenantID)
	require.Equal(t, []string{"kb:read", "doc:write"}, []string(got.Scopes))
	require.Equal(t, "active", got.Status)
}

func TestAPIKeyRepo_GetByHash_NotFound(t *testing.T) {
	db := newAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)

	got, err := repo.GetByHash(context.Background(), "no-such-hash")
	require.Error(t, err, "missing key should error (not nil)")
	require.Nil(t, got)
}

func TestAPIKeyRepo_GetByHash_SkipsRevoked(t *testing.T) {
	db := newAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &types.APIKey{
		ID: "ak_2", TenantID: 1, KeyHash: "h2", Status: "revoked", Scopes: types.StringArray{"kb:read"},
	}))
	// A revoked key must NOT be returned by GetByHash — a revoked credential
	// must not authenticate even if its hash matches.
	got, err := repo.GetByHash(ctx, "h2")
	require.Error(t, err)
	require.Nil(t, got)
}

func TestAPIKeyRepo_ListByTenant(t *testing.T) {
	db := newAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &types.APIKey{ID: "a", TenantID: 5, KeyHash: "ha", Scopes: types.StringArray{"kb:read"}}))
	require.NoError(t, repo.Create(ctx, &types.APIKey{ID: "b", TenantID: 5, KeyHash: "hb", Scopes: types.StringArray{"chat"}}))
	require.NoError(t, repo.Create(ctx, &types.APIKey{ID: "c", TenantID: 9, KeyHash: "hc", Scopes: types.StringArray{"kb:read"}}))

	keys, err := repo.ListByTenant(ctx, 5)
	require.NoError(t, err)
	require.Len(t, keys, 2, "only tenant 5 keys")
}

func TestAPIKeyRepo_Revoke(t *testing.T) {
	db := newAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &types.APIKey{ID: "ak_r", TenantID: 1, KeyHash: "hr", Status: "active", Scopes: types.StringArray{"kb:read"}}))

	require.NoError(t, repo.Revoke(ctx, "ak_r"))

	// Revoked -> no longer retrievable by hash (auth path treats it as gone).
	got, err := repo.GetByHash(ctx, "hr")
	require.Error(t, err)
	require.Nil(t, got)
}

func TestAPIKeyRepo_TouchLastUsed(t *testing.T) {
	db := newAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &types.APIKey{ID: "ak_u", TenantID: 1, KeyHash: "hu", Status: "active", Scopes: types.StringArray{"kb:read"}}))
	before, err := repo.GetByHash(ctx, "hu")
	require.NoError(t, err)
	require.Nil(t, before.LastUsedAt)

	// TouchLastUsed updates last_used_at; best-effort, must not error.
	require.NoError(t, repo.TouchLastUsed(ctx, "ak_u", time.Now()))

	// Re-read via a direct fetch (GetByHash filters only on status, not time).
	var reloaded types.APIKey
	require.NoError(t, db.First(&reloaded, "id = ?", "ak_u").Error)
	require.NotNil(t, reloaded.LastUsedAt, "last_used_at should be set")
}

func TestAPIKeyRepo_GetByID(t *testing.T) {
	db := newAPIKeyTestDB(t)
	repo := NewAPIKeyRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &types.APIKey{ID: "ak_g", TenantID: 2, KeyHash: "hg", Scopes: types.StringArray{"doc:read"}}))

	got, err := repo.GetByID(ctx, "ak_g")
	require.NoError(t, err)
	require.Equal(t, "ak_g", got.ID)
	// GetByID returns revoked keys too (it's the management read path, not auth).
	got.Status = "revoked"
	require.NoError(t, db.Save(got).Error)
	got2, err := repo.GetByID(ctx, "ak_g")
	require.NoError(t, err)
	require.Equal(t, "revoked", got2.Status)
}
