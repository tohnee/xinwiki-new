package service

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	"github.com/google/uuid"
)

// errDBRepoNotConfigured is the sentinel returned by the constructor helpers
// below when a DB-backed path is invoked without a DB repo wired in. This
// never happens in production (dig wires the repo) but keeps the failure
// mode unambiguous if a future refactor forgets the wiring.
var errDBRepoNotConfigured = fmt.Errorf("prompt template: DB repo not configured")

type promptTemplateRepo struct {
	mu        sync.RWMutex
	templates map[string]map[uint64]map[string]*types.PromptTemplate
}

var promptRepoInstance *promptTemplateRepo
var promptRepoOnce sync.Once

func getPromptTemplateRepo() *promptTemplateRepo {
	promptRepoOnce.Do(func() {
		promptRepoInstance = &promptTemplateRepo{
			templates: make(map[string]map[uint64]map[string]*types.PromptTemplate),
		}
		promptRepoInstance.initDefaultTemplates()
	})
	return promptRepoInstance
}

// PromptTemplateServiceImpl implements interfaces.PromptTemplateService.
//
// Two storage backends are supported via the (mutually-exclusive) `dbRepo`
// vs `getPromptTemplateRepo()` paths:
//
//   - DB-backed (production): when NewPromptTemplateService is called with
//     a non-nil `interfaces.PromptTemplateRepository`, every method flows
//     through the DB repo. Templates survive process restarts and are
//     visible to every replica, which unblocks multi-replica HA.
//   - In-memory (tests / no-DB Lite): when called with `nil`, every method
//     falls back to the pre-existing package-level singleton.
//
// The dual path keeps the Lite single-binary / unit-test bootpaths that
// never built the DB repo working without changing test fakes.
type PromptTemplateServiceImpl struct {
	// dbRepo is nil when running against the in-memory fallback.
	dbRepo interfaces.PromptTemplateRepository
}

// NewPromptTemplateService constructs a prompt template service. The first
// optional argument is the DB-backed repository; when nil the service
// transparently uses the in-memory fallback (unit tests, Lite no-DB boot).
func NewPromptTemplateService(dbRepo interfaces.PromptTemplateRepository) interfaces.PromptTemplateService {
	svc := &PromptTemplateServiceImpl{dbRepo: dbRepo}
	if dbRepo != nil {
		// Idempotently seed the DB with the built-in default prompts so the
		// service has something useful even on a freshly migrated database
		// before the operator edits anything. InitDefaults is a no-op when
		// the rows already exist. We use context.Background() here because
		// this runs at dig-graph construction time, not under a request
		// context; failure is logged at WARN and ignored (best-effort)
		// because blocking boot on prompt-seeding would be a poor trade.
		ctx := context.Background()
		if err := dbRepo.InitDefaults(ctx); err != nil {
			logger.Warnf(ctx, "[PromptTemplate] InitDefaults on startup failed: %v (continuing with empty DB)", err)
		}
	}
	return svc
}

func (r *promptTemplateRepo) initDefaultTemplates() {
	defaults := map[string]string{
		"system.chat":              "你是一个智能助手，请根据用户的问题提供准确、有用的回答。",
		"system.knowledge_qa":      `你是一个专业的知识库问答助手。请基于以下提供的参考资料回答用户问题。
如果参考资料中没有相关信息，请明确说明你无法从提供的资料中找到答案，不要编造信息。
回答时请标注引用来源，使用[1]、[2]这样的标记。

参考资料：
{{.context}}

用户问题：{{.question}}`,
		"system.summary":           "请对以下内容进行简洁准确的总结，保留关键信息。\n\n内容：{{.content}}",
		"system.extraction":        "请从以下文本中提取关键信息，以JSON格式输出。\n\n文本：{{.content}}",
		"system.classification":    "请将以下内容分类到预定义类别中。类别：{{.categories}}\n\n内容：{{.content}}",
		"system.rewrite":           "请改写以下内容，使其更加{{.style}}。\n\n原文：{{.content}}",
	}
	for key, content := range defaults {
		t := &types.PromptTemplate{
			ID:          uuid.New().String(),
			TemplateKey: key,
			TenantID:    0,
			Version:     "v1.0",
			Content:     content,
			Description: fmt.Sprintf("Default template for %s", key),
			IsActive:    true,
			CreatedBy:   "system",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		r.templates[key] = map[uint64]map[string]*types.PromptTemplate{
			0: {"v1.0": t},
		}
	}
}

func (s *PromptTemplateServiceImpl) CreateTemplate(ctx context.Context, tpl *types.PromptTemplate) error {
	if tpl == nil {
		return fmt.Errorf("template cannot be nil")
	}
	if tpl.TemplateKey == "" {
		return fmt.Errorf("template_key is required")
	}
	if tpl.Version == "" {
		return fmt.Errorf("version is required")
	}
	if tpl.Content == "" {
		return fmt.Errorf("content cannot be empty")
	}
	if s.dbRepo != nil {
		return s.dbRepo.Create(ctx, tpl)
	}

	tpl.ID = uuid.New().String()
	tpl.CreatedAt = time.Now()
	tpl.UpdatedAt = time.Now()

	repo := getPromptTemplateRepo()
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if _, ok := repo.templates[tpl.TemplateKey]; !ok {
		repo.templates[tpl.TemplateKey] = make(map[uint64]map[string]*types.PromptTemplate)
	}
	if _, ok := repo.templates[tpl.TemplateKey][tpl.TenantID]; !ok {
		repo.templates[tpl.TemplateKey][tpl.TenantID] = make(map[string]*types.PromptTemplate)
	}
	if _, ok := repo.templates[tpl.TemplateKey][tpl.TenantID][tpl.Version]; ok {
		return fmt.Errorf("version %s already exists for template %s", tpl.Version, tpl.TemplateKey)
	}
	repo.templates[tpl.TemplateKey][tpl.TenantID][tpl.Version] = tpl

	if tpl.IsActive {
		for ver, existing := range repo.templates[tpl.TemplateKey][tpl.TenantID] {
			if ver != tpl.Version {
				existing.IsActive = false
			}
		}
	}

	logger.Infof(ctx, "[PromptTemplate] Created template key=%s version=%s tenant=%d",
		tpl.TemplateKey, tpl.Version, tpl.TenantID)
	return nil
}

func (s *PromptTemplateServiceImpl) GetTemplate(ctx context.Context, tenantID uint64, templateKey, version string) (*types.PromptTemplate, error) {
	if s.dbRepo != nil {
		return s.dbRepo.Get(ctx, tenantID, templateKey, version)
	}
	repo := getPromptTemplateRepo()
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	if tenantTemplates, ok := repo.templates[templateKey]; ok {
		if versions, ok := tenantTemplates[tenantID]; ok {
			if tpl, ok := versions[version]; ok {
				return tpl, nil
			}
		}
		if tenantID != 0 {
			if versions, ok := tenantTemplates[0]; ok {
				if tpl, ok := versions[version]; ok {
					return tpl, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("template %s version %s not found", templateKey, version)
}

func (s *PromptTemplateServiceImpl) GetActiveTemplate(ctx context.Context, tenantID uint64, templateKey string) (*types.PromptTemplate, error) {
	if s.dbRepo != nil {
		return s.dbRepo.GetActive(ctx, tenantID, templateKey)
	}
	repo := getPromptTemplateRepo()
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	searchTenants := []uint64{tenantID}
	if tenantID != 0 {
		searchTenants = append(searchTenants, 0)
	}

	for _, tid := range searchTenants {
		if tenantTemplates, ok := repo.templates[templateKey]; ok {
			if versions, ok := tenantTemplates[tid]; ok {
				var latestActive *types.PromptTemplate
				var latestTime time.Time
				for _, tpl := range versions {
					if tpl.IsActive && tpl.UpdatedAt.After(latestTime) {
						latestActive = tpl
						latestTime = tpl.UpdatedAt
					}
				}
				if latestActive != nil {
					return latestActive, nil
				}
				var latest *types.PromptTemplate
				for _, tpl := range versions {
					if latest == nil || tpl.CreatedAt.After(latest.CreatedAt) {
						latest = tpl
					}
				}
				if latest != nil {
					return latest, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no active template found for %s", templateKey)
}

func (s *PromptTemplateServiceImpl) ListTemplateVersions(ctx context.Context, tenantID uint64, templateKey string) ([]*types.PromptTemplate, error) {
	if s.dbRepo != nil {
		return s.dbRepo.ListVersions(ctx, tenantID, templateKey)
	}
	repo := getPromptTemplateRepo()
	repo.mu.RLock()
	defer repo.mu.RUnlock()

	var result []*types.PromptTemplate
	seen := make(map[string]bool)

	if tenantTemplates, ok := repo.templates[templateKey]; ok {
		if versions, ok := tenantTemplates[tenantID]; ok {
			for _, tpl := range versions {
				key := fmt.Sprintf("%s-%s", tpl.TemplateKey, tpl.Version)
				if !seen[key] {
					seen[key] = true
					result = append(result, tpl)
				}
			}
		}
		if tenantID != 0 {
			if versions, ok := tenantTemplates[0]; ok {
				for _, tpl := range versions {
					key := fmt.Sprintf("%s-%s", tpl.TemplateKey, tpl.Version)
					if !seen[key] {
						seen[key] = true
						result = append(result, tpl)
					}
				}
			}
		}
	}

	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}

func (s *PromptTemplateServiceImpl) ActivateVersion(ctx context.Context, tenantID uint64, templateKey, version string) error {
	if s.dbRepo != nil {
		return s.dbRepo.SetActive(ctx, tenantID, templateKey, version)
	}
	repo := getPromptTemplateRepo()
	repo.mu.Lock()
	defer repo.mu.Unlock()

	if tenantTemplates, ok := repo.templates[templateKey]; ok {
		if versions, ok := tenantTemplates[tenantID]; ok {
			for _, tpl := range versions {
				tpl.IsActive = false
			}
			if tpl, ok := versions[version]; ok {
				tpl.IsActive = true
				tpl.UpdatedAt = time.Now()
				logger.Infof(ctx, "[PromptTemplate] Activated %s version=%s tenant=%d", templateKey, version, tenantID)
				return nil
			}
		}
	}

	return fmt.Errorf("template %s version %s not found", templateKey, version)
}

func (s *PromptTemplateServiceImpl) RenderTemplate(
	ctx context.Context,
	tenantID uint64,
	templateKey, version string,
	vars map[string]string,
) (string, string, error) {
	var tpl *types.PromptTemplate
	var err error

	if version != "" {
		tpl, err = s.GetTemplate(ctx, tenantID, templateKey, version)
	} else {
		tpl, err = s.GetActiveTemplate(ctx, tenantID, templateKey)
	}
	if err != nil {
		return "", "", err
	}

	tmpl, err := template.New(tpl.TemplateKey).Parse(tpl.Content)
	if err != nil {
		return "", tpl.Version, fmt.Errorf("failed to parse template: %w", err)
	}

	data := make(map[string]interface{})
	for k, v := range vars {
		data[k] = v
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", tpl.Version, fmt.Errorf("failed to render template: %w", err)
	}

	rendered := buf.String()
	rendered = strings.TrimSpace(rendered)
	for strings.Contains(rendered, "\n\n\n") {
		rendered = strings.ReplaceAll(rendered, "\n\n\n", "\n\n")
	}

	logger.Debugf(ctx, "[PromptTemplate] Rendered %s version=%s vars=%v", templateKey, tpl.Version, vars)
	return rendered, tpl.Version, nil
}
