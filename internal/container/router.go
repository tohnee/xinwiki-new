package container

import (
	"context"
	"io"
	"os"
	"strings"

	"go.uber.org/dig"

	"github.com/Tencent/XinWiki/internal/application/service/file"
	chat "github.com/Tencent/XinWiki/internal/models/chat"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/router"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
	secutils "github.com/Tencent/XinWiki/internal/utils"
)

func registerRouter(c *dig.Container, ctx context.Context, redisAvailable bool) {
	// Wire the chat package's local image resolver so multimodal chat can read
	// local:// images that live under a tenant's configured storage PathPrefix
	// (which is not encoded in the local:// URL).
	must(c.Invoke(registerChatLocalImageResolver))

	// Router configuration
	logger.Debugf(ctx, "[Container] Registering router and starting task server...")
	must(c.Provide(router.NewRouter))
	if redisAvailable {
		must(c.Invoke(router.RunAsynqServer))
	} else {
		must(c.Invoke(router.RegisterSyncHandlers))
	}
}

// registerChatLocalImageResolver wires the chat package's LocalImageResolver
// hook. Stored local:// URLs are relative to the resolved storage base dir and
// do NOT encode the owning tenant's configured PathPrefix, so resolving them to
// disk bytes requires rebuilding the FileService from that tenant's storage
// config. The owning tenant is parsed from the URL's first path segment, which
// correctly handles cross-tenant shared resources (e.g. shared KB images).
func registerChatLocalImageResolver(tenantRepo interfaces.TenantRepository) {
	chat.LocalImageResolver = func(storageURL string) ([]byte, bool) {
		tenantID := secutils.ParseTenantIDFromStoragePath(storageURL)
		if tenantID == 0 {
			return nil, false
		}
		ctx := context.Background()
		tenant, err := tenantRepo.GetTenantByID(ctx, tenantID)
		if err != nil || tenant == nil {
			return nil, false
		}
		baseDir := strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR"))
		fileSvc, _, err := file.NewFileServiceFromStorageConfig("local", tenant.StorageEngineConfig, baseDir)
		if err != nil {
			return nil, false
		}
		rc, err := fileSvc.GetFile(ctx, storageURL)
		if err != nil {
			return nil, false
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, false
		}
		return data, true
	}
}
