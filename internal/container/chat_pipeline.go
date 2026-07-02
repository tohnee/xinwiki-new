package container

import (
	"context"

	"go.uber.org/dig"

	chatpipeline "github.com/Tencent/XinWiki/internal/application/service/chat_pipeline"
	"github.com/Tencent/XinWiki/internal/logger"
)

func registerChatPipeline(c *dig.Container, ctx context.Context) {
	// Chat pipeline components for processing chat requests
	logger.Debugf(ctx, "[Container] Registering chat pipeline plugins...")

	must(c.Provide(chatpipeline.NewEventManager))
	must(c.Invoke(chatpipeline.NewPluginSearch))
	must(c.Invoke(chatpipeline.NewPluginRerank))
	must(c.Invoke(chatpipeline.NewPluginWebFetch))
	must(c.Invoke(chatpipeline.NewPluginMerge))
	must(c.Invoke(chatpipeline.NewPluginDataAnalysis))
	must(c.Invoke(chatpipeline.NewPluginIntoChatMessage))
	must(c.Invoke(chatpipeline.NewPluginChatCompletion))
	must(c.Invoke(chatpipeline.NewPluginChatCompletionStream))
	must(c.Invoke(chatpipeline.NewPluginFilterTopK))
	must(c.Invoke(chatpipeline.NewPluginQueryUnderstand))
	must(c.Invoke(chatpipeline.NewPluginLoadHistory))
	must(c.Invoke(chatpipeline.NewPluginExtractEntity))
	must(c.Invoke(chatpipeline.NewPluginSearchEntity))
	must(c.Invoke(chatpipeline.NewPluginSearchParallel))
	must(c.Invoke(chatpipeline.NewPluginWikiBoost))
	must(c.Invoke(chatpipeline.NewMemoryPlugin))
	logger.Debugf(ctx, "[Container] Chat pipeline plugins registered")
}
