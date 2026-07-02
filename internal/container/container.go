// Package container implements dependency injection container setup
// Provides centralized configuration for services, repositories, and handlers
// This package is responsible for wiring up all dependencies and ensuring proper lifecycle management
package container

import (
	"context"
	"os"

	"go.uber.org/dig"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types/interfaces"
)

// BuildContainer constructs the dependency injection container
// Registers all components, services, repositories and handlers needed by the application
// Creates a fully configured application container with proper dependency resolution
// Parameters:
//   - container: Base dig container to add dependencies to
//
// Returns:
//   - Configured container with all application dependencies registered
func BuildContainer(container *dig.Container) *dig.Container {
	ctx := context.Background()
	logger.Debugf(ctx, "[Container] Starting container initialization...")

	// Register resource cleaner for proper cleanup of resources
	must(container.Provide(NewResourceCleaner, dig.As(new(interfaces.ResourceCleaner))))

	redisAvailable := os.Getenv("REDIS_ADDR") != ""

	registerInfra(container, ctx)
	registerRepositories(container, ctx)
	registerMCP(container, ctx)
	registerRetriever(container, ctx)
	registerServices(container, ctx)
	registerAgent(container, ctx)
	registerTaskQueue(container, ctx, redisAvailable)
	registerDatasource(container, ctx)
	registerChatPipeline(container, ctx)
	registerHandlers(container, ctx)
	registerRouter(container, ctx, redisAvailable)

	logger.Infof(ctx, "[Container] Container initialization completed successfully")
	return container
}

// must is a helper function for error handling
// Panics if the error is not nil, useful for configuration steps that must succeed
// Parameters:
//   - err: Error to check
func must(err error) {
	if err != nil {
		panic(err)
	}
}
