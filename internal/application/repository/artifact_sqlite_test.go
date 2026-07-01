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

// artifactsTestDDL is a SQLite-compatible subset of the production DDL
// (migrations/versioned/000067_generated_artifacts.up.sql). JSONB columns
// become TEXT; StringArray serializes as JSON TEXT.
const artifactsTestDDL = `
CREATE TABLE IF NOT EXISTS generated_artifacts (
    id                  TEXT PRIMARY KEY,
    tenant_id           INTEGER NOT NULL,
    user_id             TEXT NOT NULL,
    session_id          TEXT NOT NULL DEFAULT '',
    type                TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending',
    title               TEXT NOT NULL DEFAULT '',
    source_kb_id        TEXT NOT NULL DEFAULT '',
    source_knowledge_id TEXT NOT NULL DEFAULT '',
    source_wiki_page_id TEXT NOT NULL DEFAULT '',
    source_refs         TEXT NOT NULL DEFAULT '[]',
    storage_uri         TEXT NOT NULL DEFAULT '',
    storage_type        TEXT NOT NULL DEFAULT '',
    mime_type           TEXT NOT NULL DEFAULT '',
    size_bytes          INTEGER NOT NULL DEFAULT 0,
    sharing_policy      TEXT NOT NULL DEFAULT 'private',
    allowed_user_ids    TEXT NOT NULL DEFAULT '[]',
    metadata            TEXT NOT NULL DEFAULT '{}',
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at          DATETIME
);
`

func newArtifactTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS generated_artifacts").Error)
	require.NoError(t, db.Exec(artifactsTestDDL).Error)
	return db
}

func TestArtifactRepo_CreateAndGetByID(t *testing.T) {
	db := newArtifactTestDB(t)
	repo := NewArtifactRepository(db)
	ctx := context.Background()

	a := &types.Artifact{
		ID:            "art_1",
		TenantID:      7,
		UserID:        "u1",
		SessionID:     "sess_1",
		Type:          types.ArtifactTypePDF,
		Status:        types.ArtifactStatusPending,
		Title:         "Q3 report",
		SourceKBID:    "kb_1",
		SharingPolicy: types.ArtifactSharingPrivate,
	}
	require.NoError(t, repo.Create(ctx, a))

	got, err := repo.GetByID(ctx, "art_1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "art_1", got.ID)
	require.Equal(t, uint64(7), got.TenantID)
	require.Equal(t, "u1", got.UserID)
	require.Equal(t, types.ArtifactTypePDF, got.Type)
	require.Equal(t, types.ArtifactStatusPending, got.Status)
	require.Equal(t, types.ArtifactSharingPrivate, got.SharingPolicy)
}

func TestArtifactRepo_GetByID_NotFound(t *testing.T) {
	db := newArtifactTestDB(t)
	repo := NewArtifactRepository(db)

	got, err := repo.GetByID(context.Background(), "nope")
	require.Error(t, err)
	require.Nil(t, got)
}

func TestArtifactRepo_ListByTenant_FiltersTenantAndDeleted(t *testing.T) {
	db := newArtifactTestDB(t)
	repo := NewArtifactRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &types.Artifact{ID: "a1", TenantID: 1, UserID: "u1", Type: types.ArtifactTypePDF, Status: types.ArtifactStatusReady}))
	require.NoError(t, repo.Create(ctx, &types.Artifact{ID: "a2", TenantID: 2, UserID: "u1", Type: types.ArtifactTypePDF, Status: types.ArtifactStatusReady}))
	require.NoError(t, repo.Create(ctx, &types.Artifact{ID: "a3", TenantID: 1, UserID: "u2", Type: types.ArtifactTypeChart, Status: types.ArtifactStatusReady}))
	// Soft-deleted row must not appear.
	require.NoError(t, repo.SoftDelete(ctx, 1, "a3"))

	got, err := repo.ListByTenant(ctx, 1)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "a1", got[0].ID)
}

func TestArtifactRepo_ListBySession(t *testing.T) {
	db := newArtifactTestDB(t)
	repo := NewArtifactRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &types.Artifact{ID: "a1", TenantID: 1, UserID: "u1", SessionID: "s1", Type: types.ArtifactTypePDF, Status: types.ArtifactStatusReady}))
	require.NoError(t, repo.Create(ctx, &types.Artifact{ID: "a2", TenantID: 1, UserID: "u1", SessionID: "s2", Type: types.ArtifactTypePDF, Status: types.ArtifactStatusReady}))
	require.NoError(t, repo.Create(ctx, &types.Artifact{ID: "a3", TenantID: 1, UserID: "u1", SessionID: "s1", Type: types.ArtifactTypeChart, Status: types.ArtifactStatusReady}))

	got, err := repo.ListBySession(ctx, 1, "s1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, a := range got {
		require.Equal(t, "s1", a.SessionID)
	}
}

func TestArtifactRepo_UpdateStatus(t *testing.T) {
	db := newArtifactTestDB(t)
	repo := NewArtifactRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &types.Artifact{ID: "a1", TenantID: 1, UserID: "u1", Type: types.ArtifactTypePDF, Status: types.ArtifactStatusPending}))
	require.NoError(t, repo.UpdateStatus(ctx, "a1", types.ArtifactStatusReady, "s3://bucket/a1.pdf", 4096))

	got, err := repo.GetByID(ctx, "a1")
	require.NoError(t, err)
	require.Equal(t, types.ArtifactStatusReady, got.Status)
	require.Equal(t, "s3://bucket/a1.pdf", got.StorageURI)
	require.Equal(t, int64(4096), got.SizeBytes)
}

func TestArtifactRepo_SoftDelete_CrossTenantNoOp(t *testing.T) {
	db := newArtifactTestDB(t)
	repo := NewArtifactRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &types.Artifact{ID: "a1", TenantID: 1, UserID: "u1", Type: types.ArtifactTypePDF, Status: types.ArtifactStatusReady}))
	// Tenant 2 tries to delete tenant 1's artifact -> no-op (tenant-scoped WHERE).
	require.NoError(t, repo.SoftDelete(ctx, 2, "a1"))

	got, err := repo.GetByID(ctx, "a1")
	require.NoError(t, err)
	require.NotNil(t, got, "cross-tenant soft-delete must not drop the row")
	require.Nil(t, got.DeletedAt)

	// Owner tenant deletes -> row is soft-deleted and no longer returned by GetByID.
	require.NoError(t, repo.SoftDelete(ctx, 1, "a1"))
	_, err = repo.GetByID(ctx, "a1")
	require.Error(t, err, "soft-deleted row must not be returned")
}

// TestArtifactRepo_AllowedUserIDs_RoundTrip verifies the StringArray JSON
// serialization survives a write+read, so the explicit-sharing ACL has the
// user ids it needs at evaluation time.
func TestArtifactRepo_AllowedUserIDs_RoundTrip(t *testing.T) {
	db := newArtifactTestDB(t)
	repo := NewArtifactRepository(db)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, &types.Artifact{
		ID: "a1", TenantID: 1, UserID: "u1", Type: types.ArtifactTypePDF,
		Status: types.ArtifactStatusReady, SharingPolicy: types.ArtifactSharingExplicit,
		AllowedUserIDs: types.StringArray{"u2", "u3"},
	}))
	got, err := repo.GetByID(ctx, "a1")
	require.NoError(t, err)
	require.Equal(t, []string{"u2", "u3"}, []string(got.AllowedUserIDs))

	// And the ACL evaluates against the round-tripped list.
	require.True(t, types.CanAccessArtifact(got, "u2", types.TenantRoleViewer, false))
	require.False(t, types.CanAccessArtifact(got, "u4", types.TenantRoleViewer, false))
}

// ensure time import is used even if future edits drop a reference.
var _ = time.Now
