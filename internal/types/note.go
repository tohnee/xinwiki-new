package types

import "time"

// UserNote is a per-(user, tenant) note in the Workspace. It either pins a
// saved excerpt from a chat citation (source_excerpt + source_* populated)
// or holds a free-form note the user writes by hand (only title/content).
//
// See migration 000069 for the schema rationale (tenant-scoped, no FK to
// chat sessions / chunks so notes survive upstream deletions).
type UserNote struct {
	// ID is the UUID primary key assigned at creation time.
	ID string `json:"id" gorm:"type:varchar(36);primaryKey"`
	// UserID is the note owner. Always taken from the auth context, never
	// from the request body, so users can only create/list/edit their own
	// notes.
	UserID string `json:"user_id" gorm:"type:varchar(36);index:idx_user_notes_user_tenant_created_at,priority:1"`
	// TenantID scopes the note to the tenant the user was in when they
	// created it. Switching tenants shows a different set of notes.
	TenantID uint64 `json:"tenant_id" gorm:"index:idx_user_notes_user_tenant_created_at,priority:2;index:idx_user_notes_tenant_id"`
	// SessionID optionally links the note to the chat session it was
	// saved from, so the UI can offer "open in chat". Empty for hand-
	// written notes.
	SessionID string `json:"session_id,omitempty" gorm:"type:varchar(36);index:idx_user_notes_session_id"`
	// Title is the short headline shown in the notes list. Required.
	Title string `json:"title" gorm:"type:varchar(255);not null"`
	// Content is the free-form body the user can edit. May be empty when
	// the note is just a pinned excerpt.
	Content string `json:"content" gorm:"type:text;not null;default:''"`
	// SourceExcerpt is the cited snippet the user saved from a chat
	// citation. Empty for hand-written notes.
	SourceExcerpt string `json:"source_excerpt,omitempty" gorm:"type:text"`
	// SourceRefID is the backend identifier of the cited source (chunk
	// ID, wiki page ID). Used for "jump to source" navigation.
	SourceRefID string `json:"source_ref_id,omitempty" gorm:"type:varchar(64)"`
	// SourceTitle is the denormalised title of the cited source, captured
	// at save time so the note stays readable if the source is later
	// deleted.
	SourceTitle string `json:"source_title,omitempty" gorm:"type:varchar(512)"`
	// SourceURL is the denormalised URL of the cited source.
	SourceURL string `json:"source_url,omitempty" gorm:"type:text"`
	// CreatedAt records when the note was first saved.
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_user_notes_user_tenant_created_at,priority:3;autoCreateTime"`
	// UpdatedAt records the last edit. GORM auto-updates this on Update
	// calls that use the model (not raw Updates with column lists).
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName pins the table to the migration's exact name so GORM's
// pluraliser doesn't drift if we ever rename the struct.
func (UserNote) TableName() string {
	return "user_notes"
}
