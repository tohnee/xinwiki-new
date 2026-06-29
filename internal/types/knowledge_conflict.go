package types

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConflictSeverity represents the severity level of a knowledge conflict
type ConflictSeverity string

const (
	ConflictSeverityCritical ConflictSeverity = "critical"
	ConflictSeverityHigh     ConflictSeverity = "high"
	ConflictSeverityMedium   ConflictSeverity = "medium"
	ConflictSeverityLow      ConflictSeverity = "low"
)

// ConflictStatus represents the status of a conflict resolution
type ConflictStatus string

const (
	ConflictStatusDetected    ConflictStatus = "detected"
	ConflictStatusReviewing   ConflictStatus = "reviewing"
	ConflictStatusResolved    ConflictStatus = "resolved"
	ConflictStatusDismissed   ConflictStatus = "dismissed"
)

// ConflictType represents the type of knowledge conflict
type ConflictType string

const (
	ConflictTypeAttributeValue  ConflictType = "attribute_value"
	ConflictTypeParameterDef    ConflictType = "parameter_definition"
	ConflictTypeDefinition      ConflictType = "definition"
	ConflictTypeTemporal        ConflictType = "temporal"
	ConflictTypeNumerical       ConflictType = "numerical"
	ConflictTypeCategorical     ConflictType = "categorical"
	ConflictTypeEntityIdentity  ConflictType = "entity_identity"
	ConflictTypeRelation        ConflictType = "relation"
)

// EntityAttribute represents an entity and its attribute
type EntityAttribute struct {
	EntityType  string `json:"entity_type"`
	EntityID    string `json:"entity_id,omitempty"`
	EntityName  string `json:"entity_name,omitempty"`
	Attribute   string `json:"attribute"`
	Value       string `json:"value"`
	SourceDocID string `json:"source_doc_id,omitempty"`
	ChunkID     string `json:"chunk_id,omitempty"`
	Confidence  float64 `json:"confidence,omitempty"`
}

// Conflict represents a detected knowledge conflict
type Conflict struct {
	ID              string                 `json:"id"              gorm:"type:varchar(36);primaryKey"`
	TenantID        uint64                 `json:"tenant_id"        gorm:"index;not null"`
	KBID            string                 `json:"kb_id"            gorm:"type:varchar(36);index;not null"`
	Type            ConflictType           `json:"type"            gorm:"type:varchar(32);not null"`
	Severity        ConflictSeverity       `json:"severity"        gorm:"type:varchar(16);not null"`
	Status          ConflictStatus         `json:"status"          gorm:"type:varchar(16);default:'detected';not null"`
	EntityType      string                 `json:"entity_type"      gorm:"type:varchar(64);index"`
	EntityID        string                 `json:"entity_id"        gorm:"type:varchar(36);index"`
	EntityName      string                 `json:"entity_name"      gorm:"type:varchar(256)"`
	Attribute       string                 `json:"attribute"        gorm:"type:varchar(128);index"`
	Description     string                 `json:"description"     gorm:"type:text;not null"`
	Values          []EntityAttribute      `json:"values"          gorm:"serializer:json"`
	AffectedDocs    []string               `json:"affected_docs"    gorm:"serializer:json"`
	Suggestion      string                 `json:"suggestion"      gorm:"type:text"`
	DetectedBy      string                 `json:"detected_by"      gorm:"type:varchar(32);default:'automated'"`
	ReviewerID      string                 `json:"reviewer_id"      gorm:"type:varchar(36)"`
	Resolution      string                 `json:"resolution"      gorm:"type:text"`
	ResolvedValue   string                 `json:"resolved_value"   gorm:"type:text"`
	Metadata        map[string]interface{} `json:"metadata"        gorm:"serializer:json"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	ResolvedAt      *time.Time             `json:"resolved_at,omitempty"`
	DeletedAt       gorm.DeletedAt         `json:"-"                gorm:"index"`
}

// BeforeCreate generates UUID for new conflicts
func (c *Conflict) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	if c.UpdatedAt.IsZero() {
		c.UpdatedAt = c.CreatedAt
	}
	if c.Status == "" {
		c.Status = ConflictStatusDetected
	}
	return nil
}

// BeforeUpdate sets updated_at timestamp
func (c *Conflict) BeforeUpdate(tx *gorm.DB) error {
	c.UpdatedAt = time.Now()
	if c.Status == ConflictStatusResolved && c.ResolvedAt == nil {
		now := time.Now()
		c.ResolvedAt = &now
	}
	return nil
}

// ConflictDetectionRequest represents a request to detect conflicts in a knowledge base or between documents
type ConflictDetectionRequest struct {
	TenantID       uint64   `json:"tenant_id" validate:"required"`
	KBID           string   `json:"kb_id" validate:"required"`
	DocIDs         []string `json:"doc_ids,omitempty"`
	ConflictTypes  []ConflictType `json:"conflict_types,omitempty"`
	MinConfidence  float64  `json:"min_confidence,omitempty"`
	MaxResults     int      `json:"max_results,omitempty"`
	IncludeResolved bool   `json:"include_resolved,omitempty"`
}

// ConflictDetectionResult represents the result of a conflict detection run
type ConflictDetectionResult struct {
	TotalScanned     int             `json:"total_scanned"`
	ConflictsFound   int             `json:"conflicts_found"`
	NewConflicts     int             `json:"new_conflicts"`
	ExistingConflicts int            `json:"existing_conflicts"`
	Conflicts        []*Conflict     `json:"conflicts"`
	DurationMs       int64           `json:"duration_ms"`
	RunAt            time.Time       `json:"run_at"`
}

// ConflictResolutionRequest represents a request to resolve a conflict
type ConflictResolutionRequest struct {
	ConflictID    string `json:"conflict_id" validate:"required"`
	ReviewerID    string `json:"reviewer_id" validate:"required"`
	Action        string `json:"action" validate:"required,oneof=resolve dismiss"`
	ResolvedValue string `json:"resolved_value,omitempty"`
	Resolution    string `json:"resolution,omitempty"`
}

// ConflictGovernanceSuggestion represents a governance suggestion for a conflict
type ConflictGovernanceSuggestion struct {
	ConflictID     string   `json:"conflict_id"`
	SuggestionType string   `json:"suggestion_type"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Options        []string `json:"options"`
	Recommended    string   `json:"recommended"`
	Rationale      string   `json:"rationale"`
}

// ConflictSummary provides summary statistics for conflicts
type ConflictSummary struct {
	Total             int                        `json:"total"`
	ByStatus          map[ConflictStatus]int     `json:"by_status"`
	BySeverity        map[ConflictSeverity]int   `json:"by_severity"`
	ByType            map[ConflictType]int       `json:"by_type"`
	OpenConflicts     int                        `json:"open_conflicts"`
	CriticalConflicts int                        `json:"critical_conflicts"`
	RecentConflicts   []*Conflict                `json:"recent_conflicts"`
}
