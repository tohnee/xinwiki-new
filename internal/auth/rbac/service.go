// Package rbac implements Role-Based Access Control with department-level
// permission inheritance for XinWiki multi-tenant system.
package rbac

import (
	"context"
	"sync"
	"time"

	"github.com/Tencent/XinWiki/internal/types"
	"github.com/google/uuid"
)

// ResourceType represents the type of securable resource in the system.
type ResourceType string

const (
	ResourceKnowledgeBase ResourceType = "knowledge_base"
	ResourceWikiPage      ResourceType = "wiki_page"
	ResourceAgent         ResourceType = "agent"
	ResourceDatasource    ResourceType = "datasource"
	ResourceUser          ResourceType = "user"
	ResourceRole          ResourceType = "role"
	ResourceTenant        ResourceType = "tenant"
	ResourceDepartment    ResourceType = "department"
)

// Action represents a permissible action on a resource.
type Action string

const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionExecute Action = "execute"
	ActionManage Action = "manage"
	ActionShare  Action = "share"
)

// Permission represents a specific permission granted on a resource type.
type Permission struct {
	ID           string       `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID     string       `json:"tenant_id" gorm:"index;not null"`
	Name         string       `json:"name" gorm:"not null"`
	ResourceType ResourceType `json:"resource_type" gorm:"not null"`
	Actions      []Action     `json:"actions" gorm:"serializer:json"`
	Description  string       `json:"description"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// Role represents a named collection of permissions that can be assigned to users.
type Role struct {
	ID          string        `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID    string        `json:"tenant_id" gorm:"index;not null"`
	Name        string        `json:"name" gorm:"not null"`
	Description string        `json:"description"`
	IsSystem    bool          `json:"is_system"` // System roles cannot be deleted
	Permissions []*Permission `json:"permissions" gorm:"many2many:role_permissions;"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Department represents an organizational unit in the tenant hierarchy.
type Department struct {
	ID        string        `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID  string        `json:"tenant_id" gorm:"index;not null"`
	ParentID  string        `json:"parent_id" gorm:"index;default:''"`
	Name      string        `json:"name" gorm:"not null"`
	Path      string        `json:"path" gorm:"index"` // Materialized path: /dept1/dept2/dept3
	Level     int           `json:"level"`
	SortOrder int           `json:"sort_order" gorm:"default:0"`
	Roles     []*Role       `json:"roles" gorm:"many2many:department_roles;"`
	Children  []*Department `json:"children,omitempty" gorm:"-"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// UserRoleAssignment represents the assignment of a role to a user, optionally scoped to a department.
type UserRoleAssignment struct {
	ID           string    `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID     string    `json:"tenant_id" gorm:"index;not null"`
	UserID       string    `json:"user_id" gorm:"index;not null"`
	RoleID       string    `json:"role_id" gorm:"index;not null"`
	DepartmentID string    `json:"department_id" gorm:"index;default:''"` // Empty means tenant-wide
	IsInherited  bool      `json:"is_inherited" gorm:"default:false"`     // Inherited from department
	AssignedBy   string    `json:"assigned_by"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ResourceACL represents explicit access control list entries on individual resources.
type ResourceACL struct {
	ID           string       `json:"id" gorm:"primaryKey;type:uuid"`
	TenantID     string       `json:"tenant_id" gorm:"index;not null"`
	ResourceType ResourceType `json:"resource_type" gorm:"index;not null"`
	ResourceID   string       `json:"resource_id" gorm:"index;not null"`
	UserID       string       `json:"user_id,omitempty" gorm:"index"`         // Grant to specific user
	DepartmentID string       `json:"department_id,omitempty" gorm:"index"`   // Grant to department
	RoleID       string       `json:"role_id,omitempty" gorm:"index"`         // Grant to role
	Actions      []Action     `json:"actions" gorm:"serializer:json"`
	GrantedBy    string       `json:"granted_by"`
	ExpiresAt    *time.Time   `json:"expires_at,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// PermissionCheck represents a request to check permissions.
type PermissionCheck struct {
	TenantID     string
	UserID       string
	DepartmentIDs []string // All departments the user belongs to
	ResourceType ResourceType
	ResourceID   string
	Action       Action
}

// EvaluationResult contains the result of a permission evaluation.
type EvaluationResult struct {
	Allowed      bool     `json:"allowed"`
	GrantedBy    string   `json:"granted_by,omitempty"` // role/department/resource_acl/owner/system
	MatchedRoles []string `json:"matched_roles,omitempty"`
	Reason       string   `json:"reason,omitempty"`
}

// Service defines the interface for RBAC operations.
type Service interface {
	// Role management
	CreateRole(ctx context.Context, role *Role) error
	UpdateRole(ctx context.Context, role *Role) error
	DeleteRole(ctx context.Context, tenantID, roleID string) error
	GetRole(ctx context.Context, tenantID, roleID string) (*Role, error)
	ListRoles(ctx context.Context, tenantID string) ([]*Role, error)

	// Permission management
	CreatePermission(ctx context.Context, perm *Permission) error
	ListPermissions(ctx context.Context, tenantID string) ([]*Permission, error)
	AssignPermissionToRole(ctx context.Context, tenantID, roleID, permissionID string) error
	RemovePermissionFromRole(ctx context.Context, tenantID, roleID, permissionID string) error

	// Department management
	CreateDepartment(ctx context.Context, dept *Department) error
	UpdateDepartment(ctx context.Context, dept *Department) error
	DeleteDepartment(ctx context.Context, tenantID, deptID string) error
	GetDepartment(ctx context.Context, tenantID, deptID string) (*Department, error)
	ListDepartments(ctx context.Context, tenantID string) ([]*Department, error)
	GetDepartmentTree(ctx context.Context, tenantID string) ([]*Department, error)
	GetUserDepartments(ctx context.Context, tenantID, userID string) ([]*Department, error)
	AddUserToDepartment(ctx context.Context, tenantID, userID, deptID string) error
	RemoveUserFromDepartment(ctx context.Context, tenantID, userID, deptID string) error

	// Role assignment
	AssignRoleToUser(ctx context.Context, assignment *UserRoleAssignment) error
	RemoveRoleFromUser(ctx context.Context, tenantID, userID, roleID, deptID string) error
	GetUserRoles(ctx context.Context, tenantID, userID string) ([]*UserRoleAssignment, error)

	// Resource ACL
	GrantAccess(ctx context.Context, acl *ResourceACL) error
	RevokeAccess(ctx context.Context, aclID string) error
	GetResourceACL(ctx context.Context, tenantID string, resourceType ResourceType, resourceID string) ([]*ResourceACL, error)

	// Permission evaluation
	CheckPermission(ctx context.Context, check *PermissionCheck) (*EvaluationResult, error)
	HasPermission(ctx context.Context, check *PermissionCheck) bool

	// Department permission inheritance
	GetInheritedPermissions(ctx context.Context, tenantID, userID string) (map[string][]Action, error)
}

// service is the concrete implementation of RBAC Service.
type service struct {
	mu            sync.RWMutex
	repo          Repository
	systemRoles   map[string]*Role // System roles initialized at startup
}

// Repository defines the data access interface for RBAC.
type Repository interface {
	// Role operations
	CreateRole(ctx context.Context, role *Role) error
	UpdateRole(ctx context.Context, role *Role) error
	DeleteRole(ctx context.Context, tenantID, roleID string) error
	GetRole(ctx context.Context, tenantID, roleID string) (*Role, error)
	ListRoles(ctx context.Context, tenantID string) ([]*Role, error)

	// Permission operations
	CreatePermission(ctx context.Context, perm *Permission) error
	ListPermissions(ctx context.Context, tenantID string) ([]*Permission, error)
	AssignPermissionToRole(ctx context.Context, roleID, permissionID string) error
	RemovePermissionFromRole(ctx context.Context, roleID, permissionID string) error

	// Department operations
	CreateDepartment(ctx context.Context, dept *Department) error
	UpdateDepartment(ctx context.Context, dept *Department) error
	DeleteDepartment(ctx context.Context, tenantID, deptID string) error
	GetDepartment(ctx context.Context, tenantID, deptID string) (*Department, error)
	ListDepartments(ctx context.Context, tenantID string) ([]*Department, error)
	GetDepartmentChildren(ctx context.Context, tenantID, parentID string) ([]*Department, error)
	GetUserDepartments(ctx context.Context, tenantID, userID string) ([]string, error)
	AddUserToDepartment(ctx context.Context, tenantID, userID, deptID string) error
	RemoveUserFromDepartment(ctx context.Context, tenantID, userID, deptID string) error

	// Role assignment operations
	AssignRoleToUser(ctx context.Context, assignment *UserRoleAssignment) error
	RemoveRoleFromUser(ctx context.Context, tenantID, userID, roleID, deptID string) error
	GetUserRoles(ctx context.Context, tenantID, userID string) ([]*UserRoleAssignment, error)
	GetDepartmentRoles(ctx context.Context, tenantID, deptID string) ([]string, error)

	// Resource ACL operations
	GrantAccess(ctx context.Context, acl *ResourceACL) error
	RevokeAccess(ctx context.Context, aclID string) error
	GetResourceACL(ctx context.Context, tenantID string, resourceType ResourceType, resourceID string) ([]*ResourceACL, error)

	// User helper
	GetUser(ctx context.Context, tenantID, userID string) (*types.User, error)
}

// NewService creates a new RBAC service instance.
func NewService(repo Repository) Service {
	svc := &service{
		repo:        repo,
		systemRoles: make(map[string]*Role),
	}
	svc.initSystemRoles()
	return svc
}

// initSystemRoles creates the built-in system roles that exist for every tenant.
func (s *service) initSystemRoles() {
	s.systemRoles = map[string]*Role{
		"owner": {
			ID:       "system-owner",
			Name:     "Owner",
			IsSystem: true,
			Permissions: []*Permission{
				{ResourceType: ResourceTenant, Actions: []Action{ActionManage}},
				{ResourceType: ResourceKnowledgeBase, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionManage, ActionShare}},
				{ResourceType: ResourceWikiPage, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionManage, ActionShare}},
				{ResourceType: ResourceAgent, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionExecute, ActionManage, ActionShare}},
				{ResourceType: ResourceDatasource, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionManage}},
				{ResourceType: ResourceUser, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionManage}},
				{ResourceType: ResourceRole, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionManage}},
				{ResourceType: ResourceDepartment, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionManage}},
			},
		},
		"admin": {
			ID:       "system-admin",
			Name:     "Admin",
			IsSystem: true,
			Permissions: []*Permission{
				{ResourceType: ResourceKnowledgeBase, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionManage, ActionShare}},
				{ResourceType: ResourceWikiPage, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionManage, ActionShare}},
				{ResourceType: ResourceAgent, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionExecute, ActionManage, ActionShare}},
				{ResourceType: ResourceDatasource, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete, ActionManage}},
				{ResourceType: ResourceUser, Actions: []Action{ActionRead, ActionUpdate, ActionManage}},
				{ResourceType: ResourceRole, Actions: []Action{ActionRead}},
				{ResourceType: ResourceDepartment, Actions: []Action{ActionRead, ActionUpdate}},
			},
		},
		"contributor": {
			ID:       "system-contributor",
			Name:     "Contributor",
			IsSystem: true,
			Permissions: []*Permission{
				{ResourceType: ResourceKnowledgeBase, Actions: []Action{ActionCreate, ActionRead, ActionUpdate}},
				{ResourceType: ResourceWikiPage, Actions: []Action{ActionCreate, ActionRead, ActionUpdate}},
				{ResourceType: ResourceAgent, Actions: []Action{ActionCreate, ActionRead, ActionUpdate, ActionExecute}},
				{ResourceType: ResourceDatasource, Actions: []Action{ActionCreate, ActionRead}},
			},
		},
		"viewer": {
			ID:       "system-viewer",
			Name:     "Viewer",
			IsSystem: true,
			Permissions: []*Permission{
				{ResourceType: ResourceKnowledgeBase, Actions: []Action{ActionRead}},
				{ResourceType: ResourceWikiPage, Actions: []Action{ActionRead}},
				{ResourceType: ResourceAgent, Actions: []Action{ActionRead, ActionExecute}},
			},
		},
	}
}

// generateID creates a new UUID for RBAC entities.
func generateID() string {
	return uuid.New().String()
}

// newID creates a new UUID and sets timestamps for a new entity.
func newID() string {
	return uuid.New().String()
}
