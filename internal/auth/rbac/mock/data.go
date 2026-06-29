package mock

import (
	"time"

	"github.com/Tencent/XinWiki/internal/auth/rbac"
	"github.com/google/uuid"
)

var (
	// MockTenant 模拟租户
	MockTenant = struct {
		ID   string
		Name string
	}{
		ID:   "tenant-demo-001",
		Name: "演示企业",
	}

	// MockUsers 模拟用户列表
	MockUsers = []struct {
		ID             string
		Username       string
		Email          string
		DisplayName    string
		DepartmentID   string
		DepartmentName string
		Avatar         string
		IsSystemAdmin  bool
	}{
		{
			ID:             "user-admin-001",
			Username:       "admin",
			Email:          "admin@xinwiki.com",
			DisplayName:    "系统管理员",
			DepartmentID:   "dept-admin",
			DepartmentName: "信息中心",
			Avatar:         "👨‍💼",
			IsSystemAdmin:  true,
		},
		{
			ID:             "user-dev-001",
			Username:       "zhangsan",
			Email:          "zhangsan@xinwiki.com",
			DisplayName:    "张三",
			DepartmentID:   "dept-engineering",
			DepartmentName: "技术研发部",
			Avatar:         "👨‍💻",
			IsSystemAdmin:  false,
		},
		{
			ID:             "user-dev-002",
			Username:       "lisi",
			Email:          "lisi@xinwiki.com",
			DisplayName:    "李四",
			DepartmentID:   "dept-engineering-sub",
			DepartmentName: "搜索算法组",
			Avatar:         "👨‍🔬",
			IsSystemAdmin:  false,
		},
		{
			ID:             "user-pm-001",
			Username:       "wangwu",
			Email:          "wangwu@xinwiki.com",
			DisplayName:    "王五",
			DepartmentID:   "dept-product",
			DepartmentName: "产品部",
			Avatar:         "👩‍💼",
			IsSystemAdmin:  false,
		},
		{
			ID:             "user-ops-001",
			Username:       "zhaoliu",
			Email:          "zhaoliu@xinwiki.com",
			DisplayName:    "赵六",
			DepartmentID:   "dept-operations",
			DepartmentName: "运维部",
			Avatar:         "🔧",
			IsSystemAdmin:  false,
		},
		{
			ID:             "user-hr-001",
			Username:       "sunqi",
			Email:          "sunqi@xinwiki.com",
			DisplayName:    "孙七",
			DepartmentID:   "dept-hr",
			DepartmentName: "人力资源部",
			Avatar:         "👩‍💻",
			IsSystemAdmin:  false,
		},
	}

	// MockDepartments 模拟部门树
	MockDepartments = []*rbac.Department{
		{
			ID:        "dept-root",
			TenantID:  MockTenant.ID,
			ParentID:  "",
			Name:      "演示企业",
			Path:      "/dept-root",
			Level:     0,
			SortOrder: 0,
			CreatedAt: time.Now().Add(-365 * 24 * time.Hour),
		},
		{
			ID:        "dept-admin",
			TenantID:  MockTenant.ID,
			ParentID:  "dept-root",
			Name:      "信息中心",
			Path:      "/dept-root/dept-admin",
			Level:     1,
			SortOrder: 1,
			CreatedAt: time.Now().Add(-365 * 24 * time.Hour),
		},
		{
			ID:        "dept-engineering",
			TenantID:  MockTenant.ID,
			ParentID:  "dept-root",
			Name:      "技术研发部",
			Path:      "/dept-root/dept-engineering",
			Level:     1,
			SortOrder: 2,
			CreatedAt: time.Now().Add(-300 * 24 * time.Hour),
		},
		{
			ID:        "dept-engineering-sub",
			TenantID:  MockTenant.ID,
			ParentID:  "dept-engineering",
			Name:      "搜索算法组",
			Path:      "/dept-root/dept-engineering/dept-engineering-sub",
			Level:     2,
			SortOrder: 1,
			CreatedAt: time.Now().Add(-200 * 24 * time.Hour),
		},
		{
			ID:        "dept-product",
			TenantID:  MockTenant.ID,
			ParentID:  "dept-root",
			Name:      "产品部",
			Path:      "/dept-root/dept-product",
			Level:     1,
			SortOrder: 3,
			CreatedAt: time.Now().Add(-280 * 24 * time.Hour),
		},
		{
			ID:        "dept-operations",
			TenantID:  MockTenant.ID,
			ParentID:  "dept-root",
			Name:      "运维部",
			Path:      "/dept-root/dept-operations",
			Level:     1,
			SortOrder: 4,
			CreatedAt: time.Now().Add(-250 * 24 * time.Hour),
		},
		{
			ID:        "dept-hr",
			TenantID:  MockTenant.ID,
			ParentID:  "dept-root",
			Name:      "人力资源部",
			Path:      "/dept-root/dept-hr",
			Level:     1,
			SortOrder: 5,
			CreatedAt: time.Now().Add(-200 * 24 * time.Hour),
		},
	}

	// MockRoles 模拟角色定义
	MockRoles = []*rbac.Role{
		{
			ID:          "role-system-admin",
			TenantID:    MockTenant.ID,
			Name:        "系统管理员",
			Description: "拥有系统所有权限",
			IsSystem:    true,
			CreatedAt:   time.Now().Add(-365 * 24 * time.Hour),
		},
		{
			ID:          "role-kb-admin",
			TenantID:    MockTenant.ID,
			Name:        "知识库管理员",
			Description: "管理知识库配置和权限",
			IsSystem:    false,
			CreatedAt:   time.Now().Add(-300 * 24 * time.Hour),
		},
		{
			ID:          "role-engineer",
			TenantID:    MockTenant.ID,
			Name:        "研发工程师",
			Description: "可以阅读和编辑技术文档",
			IsSystem:    false,
			CreatedAt:   time.Now().Add(-280 * 24 * time.Hour),
		},
		{
			ID:          "role-product-manager",
			TenantID:    MockTenant.ID,
			Name:        "产品经理",
			Description: "可以阅读和编辑产品文档",
			IsSystem:    false,
			CreatedAt:   time.Now().Add(-250 * 24 * time.Hour),
		},
		{
			ID:          "role-viewer",
			TenantID:    MockTenant.ID,
			Name:        "只读用户",
			Description: "只能查看文档，不能编辑",
			IsSystem:    false,
			CreatedAt:   time.Now().Add(-200 * 24 * time.Hour),
		},
		{
			ID:          "role-search-engineer",
			TenantID:    MockTenant.ID,
			Name:        "搜索算法工程师",
			Description: "搜索算法组专用角色，可以管理检索配置",
			IsSystem:    false,
			CreatedAt:   time.Now().Add(-180 * 24 * time.Hour),
		},
	}

	// MockPermissions 模拟权限定义
	MockPermissions = []*rbac.Permission{
		// 知识库权限
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			Name:         "知识库:完全控制",
			ResourceType: rbac.ResourceKnowledgeBase,
			Actions:      []rbac.Action{rbac.ActionCreate, rbac.ActionRead, rbac.ActionUpdate, rbac.ActionDelete, rbac.ActionManage, rbac.ActionShare},
			Description:  "创建、查看、编辑、删除、管理、分享知识库",
		},
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			Name:         "知识库:查看",
			ResourceType: rbac.ResourceKnowledgeBase,
			Actions:      []rbac.Action{rbac.ActionRead},
			Description:  "查看知识库列表和基本信息",
		},
		// Wiki页面权限
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			Name:         "Wiki页面:编辑",
			ResourceType: rbac.ResourceWikiPage,
			Actions:      []rbac.Action{rbac.ActionCreate, rbac.ActionRead, rbac.ActionUpdate, rbac.ActionDelete, rbac.ActionShare},
			Description:  "创建、查看、编辑、删除、分享Wiki页面",
		},
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			Name:         "Wiki页面:查看",
			ResourceType: rbac.ResourceWikiPage,
			Actions:      []rbac.Action{rbac.ActionRead},
			Description:  "查看Wiki页面内容",
		},
		// Agent权限
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			Name:         "Agent:执行",
			ResourceType: rbac.ResourceAgent,
			Actions:      []rbac.Action{rbac.ActionRead, rbac.ActionExecute},
			Description:  "查看和执行AI Agent",
		},
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			Name:         "Agent:管理",
			ResourceType: rbac.ResourceAgent,
			Actions:      []rbac.Action{rbac.ActionCreate, rbac.ActionRead, rbac.ActionUpdate, rbac.ActionDelete, rbac.ActionManage},
			Description:  "创建和管理AI Agent配置",
		},
		// 用户和角色管理权限
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			Name:         "用户管理",
			ResourceType: rbac.ResourceUser,
			Actions:      []rbac.Action{rbac.ActionCreate, rbac.ActionRead, rbac.ActionUpdate, rbac.ActionDelete, rbac.ActionManage},
			Description:  "管理用户账号",
		},
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			Name:         "角色管理",
			ResourceType: rbac.ResourceRole,
			Actions:      []rbac.Action{rbac.ActionCreate, rbac.ActionRead, rbac.ActionUpdate, rbac.ActionDelete, rbac.ActionManage},
			Description:  "管理角色和权限配置",
		},
		// 部门管理权限
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			Name:         "部门管理",
			ResourceType: rbac.ResourceDepartment,
			Actions:      []rbac.Action{rbac.ActionCreate, rbac.ActionRead, rbac.ActionUpdate, rbac.ActionDelete, rbac.ActionManage},
			Description:  "管理组织架构",
		},
	}

	// MockUserRoles 用户-角色分配
	MockUserRoles = []*rbac.UserRoleAssignment{
		// 系统管理员拥有所有权限
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			UserID:       "user-admin-001",
			RoleID:       "role-system-admin",
			DepartmentID: "",
			IsInherited:  false,
			AssignedBy:   "system",
			CreatedAt:    time.Now().Add(-365 * 24 * time.Hour),
		},
		// 张三：技术研发部直接分配研发工程师角色
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			UserID:       "user-dev-001",
			RoleID:       "role-engineer",
			DepartmentID: "dept-engineering",
			IsInherited:  false,
			AssignedBy:   "user-admin-001",
			CreatedAt:    time.Now().Add(-280 * 24 * time.Hour),
		},
		// 李四：搜索算法组，继承技术研发部的研发工程师角色 + 直接分配搜索算法工程师角色
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			UserID:       "user-dev-002",
			RoleID:       "role-engineer",
			DepartmentID: "dept-engineering-sub",
			IsInherited:  true,
			AssignedBy:   "department-inheritance",
			CreatedAt:    time.Now().Add(-200 * 24 * time.Hour),
		},
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			UserID:       "user-dev-002",
			RoleID:       "role-search-engineer",
			DepartmentID: "dept-engineering-sub",
			IsInherited:  false,
			AssignedBy:   "user-admin-001",
			CreatedAt:    time.Now().Add(-180 * 24 * time.Hour),
		},
		// 王五：产品部，产品经理角色
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			UserID:       "user-pm-001",
			RoleID:       "role-product-manager",
			DepartmentID: "dept-product",
			IsInherited:  false,
			AssignedBy:   "user-admin-001",
			CreatedAt:    time.Now().Add(-250 * 24 * time.Hour),
		},
		// 赵六：运维部，只读用户角色
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			UserID:       "user-ops-001",
			RoleID:       "role-viewer",
			DepartmentID: "dept-operations",
			IsInherited:  false,
			AssignedBy:   "user-admin-001",
			CreatedAt:    time.Now().Add(-200 * 24 * time.Hour),
		},
		// 孙七：人力资源部，只读用户角色
		{
			ID:           uuid.New().String(),
			TenantID:     MockTenant.ID,
			UserID:       "user-hr-001",
			RoleID:       "role-viewer",
			DepartmentID: "dept-hr",
			IsInherited:  false,
			AssignedBy:   "user-admin-001",
			CreatedAt:    time.Now().Add(-200 * 24 * time.Hour),
		},
	}

	// MockDepartmentRoles 部门-角色分配（权限继承来源）
	MockDepartmentRoles = map[string][]string{
		"dept-admin":         {"role-system-admin"},
		"dept-engineering":   {"role-engineer"},
		"dept-product":       {"role-product-manager"},
		"dept-operations":    {"role-viewer"},
		"dept-hr":            {"role-viewer"},
	}
)
