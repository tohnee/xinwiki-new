package container

import (
	"context"

	"go.uber.org/dig"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/mcp"
)

func registerMCP(c *dig.Container, ctx context.Context) {
	// MCP manager for managing MCP client connections
	logger.Debugf(ctx, "[Container] Registering MCP manager...")
	must(c.Provide(mcp.NewMCPManager))
	must(c.Provide(mcp.NewOAuthManager))
}
