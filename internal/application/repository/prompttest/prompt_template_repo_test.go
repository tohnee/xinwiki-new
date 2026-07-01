package prompttest

import (
	"context"
	"testing"

	repo "github.com/Tencent/XinWiki/internal/application/repository"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.PromptTemplate{}))
	return db
}

func TestPromptTemplateRepo_Create(t *testing.T) {
	db := setupDB(t)
	repo := repo.NewPromptTemplateRepository(db)
	ctx := context.Background()

	tpl := &types.PromptTemplate{
		ID:          uuid.New().String(),
		TemplateKey: "system.test",
		TenantID:    0,
		Version:     "v1.0",
		Content:     "Hello {{.name}}",
		IsActive:    true,
		CreatedBy:   "test",
	}

	err := repo.Create(ctx, tpl)
	assert.NoError(t, err)

	// Verify it was persisted
	got, err := repo.Get(ctx, 0, "system.test", "v1.0")
	assert.NoError(t, err)
	assert.Equal(t, "Hello {{.name}}", got.Content)
	assert.Equal(t, "v1.0", got.Version)
}

func TestPromptTemplateRepo_DuplicateVersion(t *testing.T) {
	db := setupDB(t)
	repo := repo.NewPromptTemplateRepository(db)
	ctx := context.Background()

	tpl := &types.PromptTemplate{
		ID:          uuid.New().String(),
		TemplateKey: "system.test",
		TenantID:    0,
		Version:     "v1.0",
		Content:     "content",
		IsActive:    true,
	}
	require.NoError(t, repo.Create(ctx, tpl))

	// Same key + tenant + version should fail
	dup := &types.PromptTemplate{
		ID:          uuid.New().String(),
		TemplateKey: "system.test",
		TenantID:    0,
		Version:     "v1.0",
		Content:     "different",
		IsActive:    false,
	}
	err := repo.Create(ctx, dup)
	assert.Error(t, err)
}

func TestPromptTemplateRepo_GetActive(t *testing.T) {
	db := setupDB(t)
	repo := repo.NewPromptTemplateRepository(db)
	ctx := context.Background()

	// Create v1.0 (inactive) and v2.0 (active)
	repo.Create(ctx, &types.PromptTemplate{
		ID: uuid.New().String(), TemplateKey: "system.qa", TenantID: 0,
		Version: "v1.0", Content: "old", IsActive: false, CreatedBy: "test",
	})
	repo.Create(ctx, &types.PromptTemplate{
		ID: uuid.New().String(), TemplateKey: "system.qa", TenantID: 0,
		Version: "v2.0", Content: "new", IsActive: true, CreatedBy: "test",
	})

	got, err := repo.GetActive(ctx, 0, "system.qa")
	assert.NoError(t, err)
	assert.Equal(t, "v2.0", got.Version)
	assert.Equal(t, "new", got.Content)
}

func TestPromptTemplateRepo_GetActive_FallbackToSystem(t *testing.T) {
	db := setupDB(t)
	repo := repo.NewPromptTemplateRepository(db)
	ctx := context.Background()

	// Only system-level (tenant_id=0) template exists
	repo.Create(ctx, &types.PromptTemplate{
		ID: uuid.New().String(), TemplateKey: "system.chat", TenantID: 0,
		Version: "v1.0", Content: "system-level", IsActive: true, CreatedBy: "test",
	})

	// Tenant 1 should fall back to system level
	got, err := repo.GetActive(ctx, 1, "system.chat")
	assert.NoError(t, err)
	assert.Equal(t, "system-level", got.Content)
}

func TestPromptTemplateRepo_ListVersions(t *testing.T) {
	db := setupDB(t)
	repo := repo.NewPromptTemplateRepository(db)
	ctx := context.Background()

	repo.Create(ctx, &types.PromptTemplate{
		ID: uuid.New().String(), TemplateKey: "system.list", TenantID: 0,
		Version: "v1.0", Content: "first", IsActive: false, CreatedBy: "test",
	})
	repo.Create(ctx, &types.PromptTemplate{
		ID: uuid.New().String(), TemplateKey: "system.list", TenantID: 0,
		Version: "v2.0", Content: "second", IsActive: true, CreatedBy: "test",
	})
	repo.Create(ctx, &types.PromptTemplate{
		ID: uuid.New().String(), TemplateKey: "system.list", TenantID: 0,
		Version: "v3.0", Content: "third", IsActive: false, CreatedBy: "test",
	})

	versions, err := repo.ListVersions(ctx, 0, "system.list")
	assert.NoError(t, err)
	assert.Len(t, versions, 3)
}

func TestPromptTemplateRepo_ActivateVersion(t *testing.T) {
	db := setupDB(t)
	repo := repo.NewPromptTemplateRepository(db)
	ctx := context.Background()

	repo.Create(ctx, &types.PromptTemplate{
		ID: uuid.New().String(), TemplateKey: "system.act", TenantID: 0,
		Version: "v1.0", Content: "old", IsActive: true, CreatedBy: "test",
	})
	repo.Create(ctx, &types.PromptTemplate{
		ID: uuid.New().String(), TemplateKey: "system.act", TenantID: 0,
		Version: "v2.0", Content: "new", IsActive: false, CreatedBy: "test",
	})

	// Activate v2.0
	err := repo.Activate(ctx, 0, "system.act", "v2.0")
	assert.NoError(t, err)

	// v1.0 should now be inactive
	v1, _ := repo.Get(ctx, 0, "system.act", "v1.0")
	assert.False(t, v1.IsActive)

	// v2.0 should be active
	v2, _ := repo.Get(ctx, 0, "system.act", "v2.0")
	assert.True(t, v2.IsActive)
}

func TestPromptTemplateRepo_InitDefaults(t *testing.T) {
	db := setupDB(t)
	repo := repo.NewPromptTemplateRepository(db)
	ctx := context.Background()

	// Initialize defaults
	err := repo.InitDefaults(ctx)
	assert.NoError(t, err)

	// Verify a default template exists
	got, err := repo.GetActive(ctx, 0, "system.chat")
	assert.NoError(t, err)
	assert.NotEmpty(t, got.Content)
}

func TestPromptTemplateRepo_PersistenceAcrossInstances(t *testing.T) {
	db := setupDB(t)

	// First instance creates a template
	repo1 := repo.NewPromptTemplateRepository(db)
	ctx := context.Background()
	repo1.Create(ctx, &types.PromptTemplate{
		ID: uuid.New().String(), TemplateKey: "system.persist", TenantID: 0,
		Version: "v1.0", Content: "persists", IsActive: true, CreatedBy: "test",
	})

	// Second instance (simulating restart) should see the same data
	repo2 := repo.NewPromptTemplateRepository(db)
	got, err := repo2.Get(ctx, 0, "system.persist", "v1.0")
	assert.NoError(t, err)
	assert.Equal(t, "persists", got.Content)
}
