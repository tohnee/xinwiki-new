package rbac

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
)

// --- Role Management ---

func (s *service) CreateRole(ctx context.Context, role *Role) error {
	if role.TenantID == "" || role.Name == "" {
		return errors.New("tenant_id and name are required")
	}
	if role.ID == "" {
		role.ID = newID()
	}
	role.IsSystem = false
	now := time.Now()
	role.CreatedAt = now
	role.UpdatedAt = now
	return s.repo.CreateRole(ctx, role)
}

func (s *service) UpdateRole(ctx context.Context, role *Role) error {
	if role.ID == "" {
		return errors.New("role id is required")
	}
	existing, err := s.repo.GetRole(ctx, role.TenantID, role.ID)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return errors.New("cannot modify system roles")
	}
	role.UpdatedAt = time.Now()
	return s.repo.UpdateRole(ctx, role)
}

func (s *service) DeleteRole(ctx context.Context, tenantID, roleID string) error {
	existing, err := s.repo.GetRole(ctx, tenantID, roleID)
	if err != nil {
		return err
	}
	if existing.IsSystem {
		return errors.New("cannot delete system roles")
	}
	return s.repo.DeleteRole(ctx, tenantID, roleID)
}

func (s *service) GetRole(ctx context.Context, tenantID, roleID string) (*Role, error) {
	// Check system roles first
	if sysRole, ok := s.systemRoles[roleID]; ok {
		return sysRole, nil
	}
	return s.repo.GetRole(ctx, tenantID, roleID)
}

func (s *service) ListRoles(ctx context.Context, tenantID string) ([]*Role, error) {
	roles := make([]*Role, 0, len(s.systemRoles))
	for _, r := range s.systemRoles {
		// Copy system roles with tenant context
		sysRoleCopy := *r
		sysRoleCopy.TenantID = tenantID
		roles = append(roles, &sysRoleCopy)
	}
	customRoles, err := s.repo.ListRoles(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return append(roles, customRoles...), nil
}

// --- Permission Management ---

func (s *service) CreatePermission(ctx context.Context, perm *Permission) error {
	if perm.TenantID == "" || perm.Name == "" || perm.ResourceType == "" {
		return errors.New("tenant_id, name, and resource_type are required")
	}
	if perm.ID == "" {
		perm.ID = newID()
	}
	now := time.Now()
	perm.CreatedAt = now
	perm.UpdatedAt = now
	return s.repo.CreatePermission(ctx, perm)
}

func (s *service) ListPermissions(ctx context.Context, tenantID string) ([]*Permission, error) {
	return s.repo.ListPermissions(ctx, tenantID)
}

func (s *service) AssignPermissionToRole(ctx context.Context, tenantID, roleID, permissionID string) error {
	return s.repo.AssignPermissionToRole(ctx, roleID, permissionID)
}

func (s *service) RemovePermissionFromRole(ctx context.Context, tenantID, roleID, permissionID string) error {
	return s.repo.RemovePermissionFromRole(ctx, roleID, permissionID)
}

// --- Department Management ---

func (s *service) CreateDepartment(ctx context.Context, dept *Department) error {
	if dept.TenantID == "" || dept.Name == "" {
		return errors.New("tenant_id and name are required")
	}
	if dept.ID == "" {
		dept.ID = newID()
	}
	now := time.Now()
	dept.CreatedAt = now
	dept.UpdatedAt = now

	// Build materialized path
	if dept.ParentID == "" || dept.ParentID == "root" {
		dept.ParentID = ""
		dept.Path = "/" + dept.ID
		dept.Level = 1
	} else {
		parent, err := s.repo.GetDepartment(ctx, dept.TenantID, dept.ParentID)
		if err != nil {
			return err
		}
		dept.Path = parent.Path + "/" + dept.ID
		dept.Level = parent.Level + 1
	}

	return s.repo.CreateDepartment(ctx, dept)
}

func (s *service) UpdateDepartment(ctx context.Context, dept *Department) error {
	if dept.ID == "" {
		return errors.New("department id is required")
	}
	dept.UpdatedAt = time.Now()
	return s.repo.UpdateDepartment(ctx, dept)
}

func (s *service) DeleteDepartment(ctx context.Context, tenantID, deptID string) error {
	// Check if department has children
	children, err := s.repo.GetDepartmentChildren(ctx, tenantID, deptID)
	if err != nil {
		return err
	}
	if len(children) > 0 {
		return errors.New("cannot delete department with children; move or delete children first")
	}
	return s.repo.DeleteDepartment(ctx, tenantID, deptID)
}

func (s *service) GetDepartment(ctx context.Context, tenantID, deptID string) (*Department, error) {
	return s.repo.GetDepartment(ctx, tenantID, deptID)
}

func (s *service) ListDepartments(ctx context.Context, tenantID string) ([]*Department, error) {
	return s.repo.ListDepartments(ctx, tenantID)
}

func (s *service) GetDepartmentTree(ctx context.Context, tenantID string) ([]*Department, error) {
	allDepts, err := s.repo.ListDepartments(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	// Build tree
	deptMap := make(map[string]*Department)
	var roots []*Department
	for _, d := range allDepts {
		deptMap[d.ID] = d
		d.Children = make([]*Department, 0)
	}
	for _, d := range allDepts {
		if d.ParentID == "" {
			roots = append(roots, d)
		} else if parent, ok := deptMap[d.ParentID]; ok {
			parent.Children = append(parent.Children, d)
		}
	}
	return roots, nil
}

func (s *service) GetUserDepartments(ctx context.Context, tenantID, userID string) ([]*Department, error) {
	deptIDs, err := s.repo.GetUserDepartments(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	depts := make([]*Department, 0, len(deptIDs))
	for _, id := range deptIDs {
		d, err := s.repo.GetDepartment(ctx, tenantID, id)
		if err != nil {
			logger.Warnf(ctx, "[rbac] failed to get department %s: %v", id, err)
			continue
		}
		depts = append(depts, d)
	}
	return depts, nil
}

func (s *service) AddUserToDepartment(ctx context.Context, tenantID, userID, deptID string) error {
	// Verify department exists
	_, err := s.repo.GetDepartment(ctx, tenantID, deptID)
	if err != nil {
		return err
	}
	return s.repo.AddUserToDepartment(ctx, tenantID, userID, deptID)
}

func (s *service) RemoveUserFromDepartment(ctx context.Context, tenantID, userID, deptID string) error {
	return s.repo.RemoveUserFromDepartment(ctx, tenantID, userID, deptID)
}

// --- Role Assignment ---

func (s *service) AssignRoleToUser(ctx context.Context, assignment *UserRoleAssignment) error {
	if assignment.TenantID == "" || assignment.UserID == "" || assignment.RoleID == "" {
		return errors.New("tenant_id, user_id, and role_id are required")
	}
	if assignment.ID == "" {
		assignment.ID = newID()
	}
	now := time.Now()
	assignment.CreatedAt = now
	assignment.UpdatedAt = now
	assignment.IsInherited = false
	return s.repo.AssignRoleToUser(ctx, assignment)
}

func (s *service) RemoveRoleFromUser(ctx context.Context, tenantID, userID, roleID, deptID string) error {
	return s.repo.RemoveRoleFromUser(ctx, tenantID, userID, roleID, deptID)
}

func (s *service) GetUserRoles(ctx context.Context, tenantID, userID string) ([]*UserRoleAssignment, error) {
	return s.repo.GetUserRoles(ctx, tenantID, userID)
}

// --- Resource ACL ---

func (s *service) GrantAccess(ctx context.Context, acl *ResourceACL) error {
	if acl.TenantID == "" || acl.ResourceType == "" || acl.ResourceID == "" || len(acl.Actions) == 0 {
		return errors.New("tenant_id, resource_type, resource_id, and actions are required")
	}
	if acl.ID == "" {
		acl.ID = newID()
	}
	now := time.Now()
	acl.CreatedAt = now
	acl.UpdatedAt = now
	return s.repo.GrantAccess(ctx, acl)
}

func (s *service) RevokeAccess(ctx context.Context, aclID string) error {
	return s.repo.RevokeAccess(ctx, aclID)
}

func (s *service) GetResourceACL(ctx context.Context, tenantID string, resourceType ResourceType, resourceID string) ([]*ResourceACL, error) {
	return s.repo.GetResourceACL(ctx, tenantID, resourceType, resourceID)
}

// --- Permission Evaluation ---

func (s *service) CheckPermission(ctx context.Context, check *PermissionCheck) (*EvaluationResult, error) {
	result := &EvaluationResult{Allowed: false}

	// 1. System admins have full access
	user, err := s.repo.GetUser(ctx, check.TenantID, check.UserID)
	if err == nil && user != nil && user.IsSystemAdmin {
		result.Allowed = true
		result.GrantedBy = "system"
		result.Reason = "system administrator"
		return result, nil
	}

	// 2. Collect all roles (direct + department inherited)
	allRoleIDs := make(map[string]string) // roleID -> source

	// Direct assignments
	assignments, err := s.repo.GetUserRoles(ctx, check.TenantID, check.UserID)
	if err != nil {
		return result, err
	}
	now := time.Now()
	for _, a := range assignments {
		if a.ExpiresAt != nil && a.ExpiresAt.Before(now) {
			continue
		}
		// Tenant-wide roles apply everywhere
		if a.DepartmentID == "" {
			allRoleIDs[a.RoleID] = "direct"
		}
	}

	// Department-inherited roles
	for _, deptID := range check.DepartmentIDs {
		deptRoleIDs, err := s.repo.GetDepartmentRoles(ctx, check.TenantID, deptID)
		if err != nil {
			logger.Warnf(ctx, "[rbac] failed to get roles for department %s: %v", deptID, err)
			continue
		}
		for _, roleID := range deptRoleIDs {
			if _, exists := allRoleIDs[roleID]; !exists {
				allRoleIDs[roleID] = "department:" + deptID
			}
		}
		// Also inherit roles from parent departments via path traversal
		dept, err := s.repo.GetDepartment(ctx, check.TenantID, deptID)
		if err != nil || dept == nil {
			continue
		}
		pathParts := strings.Split(strings.Trim(dept.Path, "/"), "/")
		for _, parentDeptID := range pathParts {
			if parentDeptID == deptID {
				continue
			}
			parentRoleIDs, err := s.repo.GetDepartmentRoles(ctx, check.TenantID, parentDeptID)
			if err != nil {
				continue
			}
			for _, roleID := range parentRoleIDs {
				if _, exists := allRoleIDs[roleID]; !exists {
					allRoleIDs[roleID] = "parent_department:" + parentDeptID
				}
			}
		}
	}

	// 3. Check permissions from roles
	for roleID, source := range allRoleIDs {
		perms := s.getRolePermissions(ctx, check.TenantID, roleID)
		for _, p := range perms {
			if p.ResourceType == check.ResourceType && containsAction(p.Actions, check.Action) {
				result.Allowed = true
				result.GrantedBy = source
				result.MatchedRoles = append(result.MatchedRoles, roleID)
				result.Reason = "role permission"
				return result, nil
			}
		}
	}

	// 4. Check resource-specific ACL
	if check.ResourceID != "" {
		aclEntries, err := s.repo.GetResourceACL(ctx, check.TenantID, check.ResourceType, check.ResourceID)
		if err == nil {
			for _, acl := range aclEntries {
				if acl.ExpiresAt != nil && acl.ExpiresAt.Before(now) {
					continue
				}
				// Check if this ACL applies to the user directly
				if acl.UserID == check.UserID && containsAction(acl.Actions, check.Action) {
					result.Allowed = true
					result.GrantedBy = "resource_acl"
					result.Reason = "explicit user grant on resource"
					return result, nil
				}
				// Check if ACL applies to any of user's departments
				if acl.DepartmentID != "" {
					for _, deptID := range check.DepartmentIDs {
						if acl.DepartmentID == deptID && containsAction(acl.Actions, check.Action) {
							result.Allowed = true
							result.GrantedBy = "resource_acl"
							result.Reason = "department grant on resource"
							return result, nil
						}
					}
				}
			}
		}
	}

	// 5. Owner check (for resources they created) - handled by middleware, not here
	result.Reason = "no matching permission found"
	return result, nil
}

func (s *service) HasPermission(ctx context.Context, check *PermissionCheck) bool {
	result, err := s.CheckPermission(ctx, check)
	if err != nil {
		logger.Warnf(ctx, "[rbac] permission check error: %v", err)
		return false
	}
	return result.Allowed
}

func (s *service) GetInheritedPermissions(ctx context.Context, tenantID, userID string) (map[string][]Action, error) {
	result := make(map[string][]Action)

	// Get all user roles
	assignments, err := s.repo.GetUserRoles(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	// Get user departments
	deptIDs, err := s.repo.GetUserDepartments(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}

	seenRoles := make(map[string]bool)

	// Add direct roles
	for _, a := range assignments {
		if !seenRoles[a.RoleID] {
			seenRoles[a.RoleID] = true
			perms := s.getRolePermissions(ctx, tenantID, a.RoleID)
			for _, p := range perms {
				key := string(p.ResourceType)
				result[key] = mergeActions(result[key], p.Actions)
			}
		}
	}

	// Add department roles (with inheritance)
	for _, deptID := range deptIDs {
		deptRoleIDs, err := s.repo.GetDepartmentRoles(ctx, tenantID, deptID)
		if err != nil {
			continue
		}
		for _, roleID := range deptRoleIDs {
			if !seenRoles[roleID] {
				seenRoles[roleID] = true
				perms := s.getRolePermissions(ctx, tenantID, roleID)
				for _, p := range perms {
					key := string(p.ResourceType)
					result[key] = mergeActions(result[key], p.Actions)
				}
			}
		}
	}

	return result, nil
}

// --- Helper functions ---

func (s *service) getRolePermissions(ctx context.Context, tenantID, roleID string) []*Permission {
	// Check system roles first
	if sysRole, ok := s.systemRoles[roleID]; ok {
		return sysRole.Permissions
	}
	// Check custom roles
	role, err := s.repo.GetRole(ctx, tenantID, roleID)
	if err != nil || role == nil {
		return nil
	}
	return role.Permissions
}

func containsAction(actions []Action, action Action) bool {
	// "manage" action implies all other actions
	for _, a := range actions {
		if a == ActionManage || a == action {
			return true
		}
	}
	return false
}

func mergeActions(existing, new []Action) []Action {
	actionSet := make(map[Action]bool)
	for _, a := range existing {
		actionSet[a] = true
	}
	for _, a := range new {
		actionSet[a] = true
	}
	merged := make([]Action, 0, len(actionSet))
	for a := range actionSet {
		merged = append(merged, a)
	}
	return merged
}
