// Package uum provides enterprise Unified User Management (UUM) integration
// supporting SCIM 2.0 for user/directory sync, SAML 2.0 for SSO, and OIDC for modern auth.
package uum

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ProviderType represents the type of enterprise identity provider.
type ProviderType string

const (
	ProviderSCIM ProviderType = "scim"
	ProviderSAML ProviderType = "saml"
	ProviderOIDC ProviderType = "oidc"
	ProviderLDAP ProviderType = "ldap"
)

// ProviderStatus represents the connection status of an identity provider.
type ProviderStatus string

const (
	StatusActive    ProviderStatus = "active"
	StatusInactive  ProviderStatus = "inactive"
	StatusPending   ProviderStatus = "pending"
	StatusError     ProviderStatus = "error"
)

// Provider represents a configured enterprise identity provider integration.
type Provider struct {
	ID             string                 `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID       string                 `json:"tenant_id" gorm:"index;not null"`
	Type           ProviderType           `json:"type" gorm:"not null"`
	Name           string                 `json:"name" gorm:"not null"`
	DisplayName    string                 `json:"display_name"`
	Status         ProviderStatus         `json:"status" gorm:"default:'pending'"`
	Config         map[string]interface{} `json:"config" gorm:"serializer:json"`
	AttributeMap   map[string]string      `json:"attribute_map" gorm:"serializer:json"` // Maps IdP attributes to XinWiki fields
	SyncEnabled    bool                   `json:"sync_enabled" gorm:"default:false"`
	SyncInterval   int                    `json:"sync_interval" gorm:"default:3600"` // Sync interval in seconds
	LastSyncAt     *time.Time             `json:"last_sync_at,omitempty"`
	LastSyncStatus string                 `json:"last_sync_status"`
	LastSyncError  string                 `json:"last_sync_error"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// SyncEvent represents a user/department synchronization event from UUM.
type SyncEvent struct {
	ID           string      `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID     string      `json:"tenant_id" gorm:"index;not null"`
	ProviderID   string      `json:"provider_id" gorm:"index;not null"`
	EventType    string      `json:"event_type" gorm:"not null"` // user.created, user.updated, user.deleted, dept.created, etc.
	ObjectType   string      `json:"object_type" gorm:"not null"` // user, department, group
	ObjectID     string      `json:"object_id" gorm:"not null"`
	ExternalID   string      `json:"external_id" gorm:"index"`
	Data         interface{} `json:"data" gorm:"serializer:json"`
	Status       string      `json:"status" gorm:"default:'pending'"` // pending, processed, failed
	ErrorMessage string      `json:"error_message"`
	ProcessedAt  *time.Time  `json:"processed_at,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
}

// SCIMUser represents a user object from SCIM 2.0 protocol.
type SCIMUser struct {
	ID          string            `json:"id"`
	ExternalID  string            `json:"externalId"`
	UserName    string            `json:"userName"`
	Name        SCIMName          `json:"name"`
	DisplayName string            `json:"displayName"`
	Emails      []SCIMEmail       `json:"emails"`
	PhoneNumbers []SCIMPhoneNumber `json:"phoneNumbers"`
	Active      bool              `json:"active"`
	Groups      []SCIMGroupRef    `json:"groups"`
	Department  string            `json:"department"` // Non-standard but common extension
	Title       string            `json:"title"`
	Meta        SCIMMeta          `json:"meta"`
}

// SCIMName represents a user's name in SCIM format.
type SCIMName struct {
	Formatted       string `json:"formatted"`
	FamilyName      string `json:"familyName"`
	GivenName       string `json:"givenName"`
	MiddleName      string `json:"middleName"`
	HonorificPrefix string `json:"honorificPrefix"`
	HonorificSuffix string `json:"honorificSuffix"`
}

// SCIMEmail represents an email address in SCIM format.
type SCIMEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type"` // work, home, other
	Primary bool   `json:"primary"`
}

// SCIMPhoneNumber represents a phone number in SCIM format.
type SCIMPhoneNumber struct {
	Value string `json:"value"`
	Type  string `json:"type"` // work, home, mobile, fax, other
}

// SCIMGroupRef represents a group reference in SCIM format.
type SCIMGroupRef struct {
	Value   string `json:"value"`
	Display string `json:"display"`
	Type    string `json:"type"` // direct, indirect
}

// SCIMDepartment represents a department/group in SCIM.
type SCIMDepartment struct {
	ID          string         `json:"id"`
	DisplayName string         `json:"displayName"`
	Members     []SCIMMemberRef `json:"members"`
	Meta        SCIMMeta       `json:"meta"`
}

// SCIMMemberRef represents a member reference in SCIM.
type SCIMMemberRef struct {
	Value   string `json:"value"`
	Display string `json:"display"`
}

// SCIMMeta represents SCIM resource metadata.
type SCIMMeta struct {
	ResourceType string    `json:"resourceType"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
	Location     string    `json:"location"`
}

// SAMLAssertion represents parsed SAML 2.0 assertion data.
type SAMLAssertion struct {
	SubjectID     string            `json:"subject_id"`
	Issuer        string            `json:"issuer"`
	SessionIndex  string            `json:"session_index"`
	Attributes    map[string]string `json:"attributes"`
	AuthnInstant  time.Time         `json:"authn_instant"`
	ExpiresAt     time.Time         `json:"expires_at"`
}

// OIDCToken represents OIDC token claims.
type OIDCToken struct {
	Issuer        string   `json:"iss"`
	Subject       string   `json:"sub"`
	Audience      string   `json:"aud"`
	Expiration    int64    `json:"exp"`
	IssuedAt      int64    `json:"iat"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Groups        []string `json:"groups"`
	Department    string   `json:"department"`
}

// SyncResult represents the result of a synchronization operation.
type SyncResult struct {
	ProviderID    string    `json:"provider_id"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at"`
	UsersCreated  int       `json:"users_created"`
	UsersUpdated  int       `json:"users_updated"`
	UsersDisabled int       `json:"users_disabled"`
	UsersDeleted  int       `json:"users_deleted"`
	DeptsCreated  int       `json:"depts_created"`
	DeptsUpdated  int       `json:"depts_updated"`
	Errors        []SyncError `json:"errors,omitempty"`
}

// SyncError represents an error during synchronization.
type SyncError struct {
	ObjectType string `json:"object_type"`
	ObjectID   string `json:"object_id"`
	Message    string `json:"message"`
}

// Service defines the interface for enterprise UUM operations.
type Service interface {
	// Provider management
	CreateProvider(ctx context.Context, provider *Provider) error
	UpdateProvider(ctx context.Context, provider *Provider) error
	DeleteProvider(ctx context.Context, tenantID, providerID string) error
	GetProvider(ctx context.Context, tenantID, providerID string) (*Provider, error)
	ListProviders(ctx context.Context, tenantID string) ([]*Provider, error)
	TestProviderConnection(ctx context.Context, tenantID, providerID string) error

	// SCIM operations
	HandleSCIMUser(ctx context.Context, tenantID string, providerType ProviderType, user *SCIMUser) error
	HandleSCIMDepartment(ctx context.Context, tenantID string, dept *SCIMDepartment) error
	SyncAllUsers(ctx context.Context, tenantID, providerID string) (*SyncResult, error)

	// SSO authentication
	ValidateSAMLAssertion(ctx context.Context, tenantID string, samlResponse string) (*SAMLAssertion, error)
	ValidateOIDCToken(ctx context.Context, tenantID string, token string) (*OIDCToken, error)
	BuildSSOURL(ctx context.Context, tenantID, providerID string, redirectURI string) (string, error)

	// User provisioning/deprovisioning
	ProvisionUser(ctx context.Context, tenantID string, userData map[string]interface{}) error
	DeprovisionUser(ctx context.Context, tenantID, externalUserID string) error
	SyncUserDepartments(ctx context.Context, tenantID, userID string, departments []string) error

	// Sync event management
	ListSyncEvents(ctx context.Context, tenantID string, limit, offset int) ([]*SyncEvent, error)
	RetrySyncEvent(ctx context.Context, tenantID, eventID string) error

	// Auto-sync
	StartSyncScheduler(ctx context.Context) error
	StopSyncScheduler(ctx context.Context) error
}

// UserRepository defines the interface required for user/department operations.
type UserRepository interface {
	// User operations
	UpsertUser(ctx context.Context, tenantID string, userData map[string]interface{}) (string, error)
	DisableUser(ctx context.Context, tenantID, externalUserID string) error
	DeleteUser(ctx context.Context, tenantID, externalUserID string) error
	FindUserByExternalID(ctx context.Context, tenantID, externalID string) (string, error)
	FindUserByEmail(ctx context.Context, email string) (string, string, error) // returns tenantID, userID

	// Department operations
	UpsertDepartment(ctx context.Context, tenantID string, deptData map[string]interface{}) (string, error)
	FindDepartmentByName(ctx context.Context, tenantID, name string) (string, error)
	FindDepartmentByExternalID(ctx context.Context, tenantID, externalID string) (string, error)
	AddUserToDepartment(ctx context.Context, tenantID, userID, deptID string) error
	RemoveUserFromDepartment(ctx context.Context, tenantID, userID, deptID string) error

	// Role assignment
	AssignDefaultRole(ctx context.Context, tenantID, userID string) error
}

// ServiceOption is a configuration option for the UUM service.
type ServiceOption func(*service)

type service struct {
	repo          Repository
	userRepo      UserRepository
	schedulers    map[string]context.CancelFunc
	httpClient    *http.Client

	// oidcValidators caches OIDC validators by providerID to avoid
	// re-fetching JWKS on every token validation request.
	oidcValidatorsMu sync.RWMutex
	oidcValidators   map[string]*oidcValidator
}

// Repository defines the data access interface for UUM providers and sync events.
type Repository interface {
	CreateProvider(ctx context.Context, provider *Provider) error
	UpdateProvider(ctx context.Context, provider *Provider) error
	DeleteProvider(ctx context.Context, tenantID, providerID string) error
	GetProvider(ctx context.Context, tenantID, providerID string) (*Provider, error)
	ListProviders(ctx context.Context, tenantID string) ([]*Provider, error)
	CreateSyncEvent(ctx context.Context, event *SyncEvent) error
	ListSyncEvents(ctx context.Context, tenantID string, limit, offset int) ([]*SyncEvent, error)
	UpdateSyncEventStatus(ctx context.Context, eventID, status string, errMsg string) error
}

// NewService creates a new UUM service instance.
func NewService(repo Repository, userRepo UserRepository, opts ...ServiceOption) Service {
	s := &service{
		repo:           repo,
		userRepo:       userRepo,
		schedulers:     make(map[string]context.CancelFunc),
		oidcValidators: make(map[string]*oidcValidator),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.httpClient == nil {
		s.httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return s
}

// WithHTTPClient sets a custom HTTP client for the UUM service (used for OIDC discovery).
func WithHTTPClient(cli *http.Client) ServiceOption {
	return func(s *service) {
		s.httpClient = cli
	}
}

// generateID creates a new UUID for UUM entities.
func generateID() string {
	return uuid.New().String()
}

// newID creates a new UUID and sets timestamps.
func newID() string {
	return uuid.New().String()
}
