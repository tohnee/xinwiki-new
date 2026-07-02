package interfaces

import (
	"context"

	"github.com/Tencent/XinWiki/internal/types"
)

// UserNoteRepository is the storage contract for per-user notes.
// Implementations are expected to be tenant-aware (queries always include
// tenant_id) and to enforce ownership: every method takes userID so a
// caller can never read or mutate another user's note, even within the
// same tenant.
type UserNoteRepository interface {
	// List returns notes for (userID, tenantID), newest first.
	List(ctx context.Context, userID string, tenantID uint64) ([]*types.UserNote, error)
	// ListBySession returns notes saved from a specific chat session.
	ListBySession(ctx context.Context, userID string, tenantID uint64, sessionID string) ([]*types.UserNote, error)
	// Get returns a single note, scoped to (userID, tenantID) so cross-
	// user access is impossible at the storage layer.
	Get(ctx context.Context, userID string, tenantID uint64, id string) (*types.UserNote, error)
	// Create persists a new note and returns the stored row (with
	// generated ID, CreatedAt, UpdatedAt).
	Create(ctx context.Context, note *types.UserNote) (*types.UserNote, error)
	// Update mutates title/content of an existing note. Source fields
	// are intentionally immutable after creation.
	Update(ctx context.Context, userID string, tenantID uint64, id string, title, content string) (*types.UserNote, error)
	// Delete removes a note. Returns whether a row was actually deleted
	// so the service can distinguish 404 from 200.
	Delete(ctx context.Context, userID string, tenantID uint64, id string) (deleted bool, err error)
}

// UserNoteService wraps the repository with input validation (non-empty
// title, length caps) and ID generation. The handler stays thin.
type UserNoteService interface {
	List(ctx context.Context, userID string, tenantID uint64) ([]*types.UserNote, error)
	ListBySession(ctx context.Context, userID string, tenantID uint64, sessionID string) ([]*types.UserNote, error)
	Get(ctx context.Context, userID string, tenantID uint64, id string) (*types.UserNote, error)
	Create(ctx context.Context, userID string, tenantID uint64, in types.UserNote) (*types.UserNote, error)
	Update(ctx context.Context, userID string, tenantID uint64, id string, title, content string) (*types.UserNote, error)
	Delete(ctx context.Context, userID string, tenantID uint64, id string) error
}
