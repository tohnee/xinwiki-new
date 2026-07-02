package im

import (
	"os"
	"strings"

	filesvc "github.com/Tencent/XinWiki/internal/application/service/file"
	"github.com/Tencent/XinWiki/internal/types"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

type imFileServiceResolver struct {
	tenant     *types.Tenant
	defaultSvc interfaces.FileService
	cache      map[string]interfaces.FileService
}

func newIMFileServiceResolver(tenant *types.Tenant, defaultSvc interfaces.FileService) *imFileServiceResolver {
	return &imFileServiceResolver{
		tenant:     tenant,
		defaultSvc: defaultSvc,
		cache:      make(map[string]interfaces.FileService),
	}
}

func (r *imFileServiceResolver) resolve(filePath string) interfaces.FileService {
	provider := types.ParseProviderScheme(filePath)
	if provider == "" {
		if r.tenant != nil && r.tenant.StorageEngineConfig != nil {
			provider = strings.ToLower(strings.TrimSpace(r.tenant.StorageEngineConfig.DefaultProvider))
		}
		if provider == "" {
			return nil
		}
	}
	if svc, ok := r.cache[provider]; ok {
		return svc
	}
	svc := buildIMFileServiceForProvider(r.tenant, provider, r.defaultSvc)
	if svc != nil {
		r.cache[provider] = svc
	}
	return svc
}

func buildIMFileServiceForProvider(
	tenant *types.Tenant,
	provider string,
	defaultSvc interfaces.FileService,
) interfaces.FileService {
	baseDir := imLocalStorageBaseDir()
	var sec *types.StorageEngineConfig
	if tenant != nil {
		sec = tenant.StorageEngineConfig
	}

	svc, _, err := filesvc.NewFileServiceFromStorageConfig(provider, sec, baseDir)
	if err == nil {
		return svc
	}
	if provider == "local" {
		externalURL := strings.TrimSpace(os.Getenv("APP_EXTERNAL_URL"))
		return filesvc.NewLocalFileService(baseDir, externalURL)
	}
	if defaultSvc != nil {
		return defaultSvc
	}
	return nil
}

func resolveIMFileServiceForPath(tenant *types.Tenant, filePath string, defaultSvc interfaces.FileService) interfaces.FileService {
	return newIMFileServiceResolver(tenant, defaultSvc).resolve(filePath)
}
