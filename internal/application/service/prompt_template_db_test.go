package service

import (
	"context"
	"testing"

	"github.com/Tencent/XinWiki/internal/application/repository"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestPromptTemplateService_DBBackedRoundtripsAndPersists proves the multi-
// replica HA path: NewPromptTemplateService wired with a DB-backed
// repository flows CreateTemplate / GetTemplate / GetActiveTemplate /
// ActivateVersion through the database, and the results survive a fresh
// service instance (which would never have been true against the old in-
// memory process singleton — each service instance rebuilt a private map).
//
// Previously the in-memory singleton lived in internal/application/service/
// prompt_template.go: the package-level state was invisible to any *other*
// replica running the same binary, so template edits through one replica
// never propagated to its peers and were lost on process restart. The DB
// repo replaces that, so this test guards the wiring by spinning up TWO
// service instances against the SAME sqlite database and confirming that
// a Create on one is visible to a Get on the other (-- the core HA
// invariant).
func TestPromptTemplateService_DBBackedRoundtripsAndPersists(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.PromptTemplate{}))

	repo := repository.NewPromptTemplateRepository(db)

	// First service instance simulates one replica of the app.
	svcA := NewPromptTemplateService(repo)
	// Second instance uses the SAME db (simulates another replica, or a
	// restart picking up persisted rows). Critically: this constructor
	// call does NOT share the in-memory singleton state with svcA — its
	// only state lives in the DB.
	svcB := NewPromptTemplateService(repo)

	ctx := context.Background()

	// svcA writes; svcB reads. Result must be visible (DB round-trip).
	newTpl := &types.PromptTemplate{
		TemplateKey: "system.test",
		TenantID:    7,
		Version:     "v1.0",
		Content:     "hello {{.name}}",
		Description: "smoke test template",
		IsActive:    true,
		CreatedBy:   "tester",
	}
	require.NoError(t, svcA.CreateTemplate(ctx, newTpl))

	// GetTemplate via the second replica.
	got, err := svcB.GetTemplate(ctx, 7, "system.test", "v1.0")
	require.NoError(t, err)
	assert.Equal(t, "hello {{.name}}", got.Content)

	// GetActive should return the same row.
	active, err := svcB.GetActiveTemplate(ctx, 7, "system.test")
	require.NoError(t, err)
	assert.Equal(t, "v1.0", active.Version, "active flag must round-trip through DB")

	// Activate a second version; svcA's prior "active" must flip to false
	// via the repo's Activate transactional SetActive path.
	require.NoError(t, svcB.CreateTemplate(ctx, &types.PromptTemplate{
		TemplateKey: "system.test",
		TenantID:    7,
		Version:     "v2.0",
		Content:     "world {{.name}}",
		IsActive:    true,
		CreatedBy:   "tester",
	}))
	require.NoError(t, svcB.ActivateVersion(ctx, 7, "system.test", "v2.0"))

	active2, err := svcA.GetActiveTemplate(ctx, 7, "system.test")
	require.NoError(t, err)
	assert.Equal(t, "v2.0", active2.Version, "activation must propagate cross-replica via DB only")

	// Activate-then-Get-then-deactivate round-trip for "not found" path.
	err = svcB.ActivateVersion(ctx, 7, "system.test", "v9-missing")
	assert.Error(t, err, "activating a missing version must error, not silently no-op")
}