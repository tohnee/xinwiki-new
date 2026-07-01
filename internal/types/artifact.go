package types

import "time"

// Generated-artifact model (review 4.2 risk 2). The right-panel generation
// surface (PPT / PDF / report / chart / diagram / markdown) needs a real
// data model with tenant / user / session / source provenance and an ACL,
// otherwise generated content becomes a cross-tenant / cross-user leak
// channel. Every artifact is scoped to a tenant, a creator, the session that
// produced it, and the source KB / document / page refs it was derived from;
// CanAccessArtifact enforces who may read it.

// ArtifactType enumerates the generated-output kinds the platform produces.
type ArtifactType string

const (
	ArtifactTypePPT      ArtifactType = "ppt"
	ArtifactTypePDF      ArtifactType = "pdf"
	ArtifactTypeReport   ArtifactType = "report"
	ArtifactTypeChart    ArtifactType = "chart"
	ArtifactTypeDiagram  ArtifactType = "diagram"
	ArtifactTypeMarkdown ArtifactType = "markdown"
)

// IsValid reports whether t is one of the declared artifact types. Used at
// creation time to reject unknown kinds before persistence.
func (t ArtifactType) IsValid() bool {
	switch t {
	case ArtifactTypePPT, ArtifactTypePDF, ArtifactTypeReport,
		ArtifactTypeChart, ArtifactTypeDiagram, ArtifactTypeMarkdown:
		return true
	}
	return false
}

// ArtifactStatus is the lifecycle state of a generated artifact.
type ArtifactStatus string

const (
	// ArtifactStatusPending means generation is in flight; the storage URI
	// is not yet populated.
	ArtifactStatusPending ArtifactStatus = "pending"
	// ArtifactStatusReady means the artifact file is materialised and ready
	// to download / preview.
	ArtifactStatusReady ArtifactStatus = "ready"
	// ArtifactStatusFailed means generation errored; the artifact row is
	// retained for audit but no file exists.
	ArtifactStatusFailed ArtifactStatus = "failed"
)

// ArtifactSharingPolicy controls visibility beyond the creator. The policy is
// evaluated by CanAccessArtifact; the tenant boundary itself is enforced by
// the repository query (ListByTenant), so within-tenant ACL reduces to these
// three policies.
type ArtifactSharingPolicy string

const (
	// ArtifactSharingPrivate: only the creator (and Admin+). Default.
	ArtifactSharingPrivate ArtifactSharingPolicy = "private"
	// ArtifactSharingTenant: visible to every member of the tenant.
	ArtifactSharingTenant ArtifactSharingPolicy = "tenant"
	// ArtifactSharingExplicit: visible to the creator plus the user ids
	// listed in AllowedUserIDs.
	ArtifactSharingExplicit ArtifactSharingPolicy = "explicit"
)

// IsValid reports whether p is one of the declared sharing policies. Used at
// creation time to reject unknown policies before persistence.
func (p ArtifactSharingPolicy) IsValid() bool {
	switch p {
	case ArtifactSharingPrivate, ArtifactSharingTenant, ArtifactSharingExplicit:
		return true
	}
	return false
}

// Artifact is a generated output produced by a chat or agent session. It
// carries the full provenance + ACL needed to keep generated content from
// leaking across tenants or users.
type Artifact struct {
	ID        string         `json:"id"          gorm:"primaryKey"`
	TenantID  uint64         `json:"tenant_id"   gorm:"index;not null"`
	UserID    string         `json:"user_id"     gorm:"not null"`       // creator
	SessionID string         `json:"session_id,omitempty" gorm:"index"` // producing chat/agent session
	Type      ArtifactType   `json:"type"        gorm:"not null"`
	Status    ArtifactStatus `json:"status"      gorm:"default:'pending'"`
	Title     string         `json:"title"`

	// Source provenance: what the artifact was generated FROM. The scalar
	// refs cover the common single-source case; SourceRefs holds richer
	// provenance (chunk ids, citations, prompt hash, ...) as JSON.
	SourceKBID        string `json:"source_kb_id,omitempty"`
	SourceKnowledgeID string `json:"source_knowledge_id,omitempty"`
	SourceWikiPageID  string `json:"source_wiki_page_id,omitempty"`
	SourceRefs        JSON   `json:"source_refs,omitempty" gorm:"type:jsonb"`

	// Storage: where the generated file lives. StorageURI is an opaque
	// object-store key / path resolved by the file service at download time.
	StorageURI  string `json:"storage_uri,omitempty"`
	StorageType string `json:"storage_type,omitempty"` // local / s3 / oss / ...
	MimeType    string `json:"mime_type,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`

	// ACL + sharing.
	SharingPolicy  ArtifactSharingPolicy `json:"sharing_policy" gorm:"default:'private'"`
	AllowedUserIDs StringArray           `json:"allowed_user_ids,omitempty" gorm:"type:jsonb"`

	// Metadata is a free-form JSON bag for generator-specific output (slide
	// count, page size, theme, render params, ...).
	Metadata JSON `json:"metadata,omitempty" gorm:"type:jsonb"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// TableName overrides the default table name for Artifact.
func (Artifact) TableName() string { return "generated_artifacts" }

// CanAccessArtifact reports whether the caller may read the artifact. It is
// fail-closed: a nil artifact, an empty caller identity, or an unknown
// sharing policy never authorizes.
//
// Evaluation order:
//  1. System admin or tenant Admin+ -> always allow (operational access;
//     the tenant boundary is already enforced by the query).
//  2. Creator -> always allow (their own artifact), provided the caller's
//     user id is non-empty so an empty==empty match can't claim ownership.
//  3. Sharing policy:
//     - "tenant"  -> any tenant member.
//     - "explicit"-> only the user ids in AllowedUserIDs.
//     - "private" -> nobody else.
//  4. Unknown policy -> deny.
func CanAccessArtifact(a *Artifact, userID string, role TenantRole, isSystemAdmin bool) bool {
	if a == nil {
		return false
	}
	if isSystemAdmin || role.HasPermission(TenantRoleAdmin) {
		return true
	}
	if userID != "" && a.UserID == userID {
		return true
	}
	switch a.SharingPolicy {
	case ArtifactSharingTenant:
		return true
	case ArtifactSharingExplicit:
		for _, u := range a.AllowedUserIDs {
			if u != "" && u == userID {
				return true
			}
		}
		return false
	case ArtifactSharingPrivate:
		return false
	}
	return false
}
