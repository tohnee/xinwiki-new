package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
)

// ConflictDetectionService handles knowledge conflict detection and management
type ConflictDetectionService interface {
	// DetectConflicts runs conflict detection on specified documents or entire KB
	DetectConflicts(ctx context.Context, req *types.ConflictDetectionRequest) (*types.ConflictDetectionResult, error)

	// GetConflict retrieves a single conflict by ID
	GetConflict(ctx context.Context, tenantID uint64, conflictID string) (*types.Conflict, error)

	// ListConflicts lists conflicts with filtering options
	ListConflicts(ctx context.Context, tenantID uint64, kbID string, status types.ConflictStatus, severity types.ConflictSeverity, conflictType types.ConflictType, page, pageSize int) ([]*types.Conflict, int, error)

	// ResolveConflict resolves or dismisses a detected conflict
	ResolveConflict(ctx context.Context, tenantID uint64, req *types.ConflictResolutionRequest) (*types.Conflict, error)

	// GetConflictSummary returns conflict summary statistics
	GetConflictSummary(ctx context.Context, tenantID uint64, kbID string) (*types.ConflictSummary, error)

	// GetGovernanceSuggestion generates governance suggestions for a conflict
	GetGovernanceSuggestion(ctx context.Context, tenantID uint64, conflictID string) (*types.ConflictGovernanceSuggestion, error)

	// DetectPairwiseConflicts detects conflicts between two specific documents
	DetectPairwiseConflicts(ctx context.Context, tenantID uint64, kbID string, docID1, docID2 string) ([]*types.Conflict, error)

	// DetectAttributeConflicts detects conflicting values for the same entity attribute
	DetectAttributeConflicts(ctx context.Context, tenantID uint64, kbID string, entityType, attribute string) ([]*types.Conflict, error)

	// DetectTemporalConflicts detects time-based conflicts (outdated information)
	DetectTemporalConflicts(ctx context.Context, tenantID uint64, kbID string, maxAge time.Duration) ([]*types.Conflict, error)
}

// ConflictRepository defines the data access layer for conflicts
type ConflictRepository interface {
	Create(ctx context.Context, conflict *types.Conflict) error
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.Conflict, error)
	List(ctx context.Context, tenantID uint64, kbID string, status types.ConflictStatus, severity types.ConflictSeverity, conflictType types.ConflictType, page, pageSize int) ([]*types.Conflict, int, error)
	Update(ctx context.Context, conflict *types.Conflict) error
	Delete(ctx context.Context, tenantID uint64, id string) error
	FindExisting(ctx context.Context, tenantID uint64, kbID string, conflictType types.ConflictType, entityType, attribute string) (*types.Conflict, error)
	CountByStatus(ctx context.Context, tenantID uint64, kbID string) (map[types.ConflictStatus]int, error)
	CountBySeverity(ctx context.Context, tenantID uint64, kbID string) (map[types.ConflictSeverity]int, error)
	CountByType(ctx context.Context, tenantID uint64, kbID string) (map[types.ConflictType]int, error)
}
