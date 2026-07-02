package im

import (
	"context"
	"fmt"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
)

func (s *Service) watchAndSendSummary(
	ctx context.Context,
	kbCtx context.Context,
	adapter Adapter,
	msg *IncomingMessage,
	knowledgeID string,
	fileName string,
	channel *IMChannel,
) {
	const (
		pollInterval = 5 * time.Second
		maxWait      = 10 * time.Minute
	)

	deadline := time.Now().Add(maxWait)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Now().After(deadline) {
				logger.Infof(ctx, "[IM] Summary watcher timed out for knowledge %s", knowledgeID)
				return
			}

			knowledge, err := s.knowledgeService.GetKnowledgeByID(kbCtx, knowledgeID)
			if err != nil {
				logger.Warnf(ctx, "[IM] Summary watcher: failed to get knowledge %s: %v", knowledgeID, err)
				return
			}

			typeName := fileTypeName(fileName)

			switch knowledge.ParseStatus {
			case types.ParseStatusFailed:
				errMsg := knowledge.ErrorMessage
				if errMsg == "" {
					errMsg = "文档解析失败"
				}
				_ = s.sendSmartReply(ctx, adapter, msg, channel,
					fmt.Sprintf("用户之前上传的%s解析失败了，错误原因：%s。请安慰用户并建议重试。", typeName, errMsg),
					fmt.Sprintf("⚠️ %s解析失败：%s", typeName, errMsg))
				return

			case types.ParseStatusCompleted:
				switch knowledge.SummaryStatus {
				case types.SummaryStatusNone, "":
					if knowledge.Description != "" && knowledge.Description != fileName {
						_ = s.sendSmartReply(ctx, adapter, msg, channel,
							fmt.Sprintf("用户之前上传的%s已解析完成。以下是文件的完整摘要内容：\n%s\n\n请生成一条通知消息，包含：1) 告知文件已解析完成；2) 用 Markdown 格式（标题、列表、加粗等）结构化展示上述摘要内容，不要删减或概括；3) 提示用户可以针对该文件提问。", typeName, knowledge.Description),
							fmt.Sprintf("📄 %s已解析完成。\n\n**摘要：**\n\n%s\n\n---\n可以针对该文件进行提问。", typeName, knowledge.Description))
					} else {
						_ = s.sendSmartReply(ctx, adapter, msg, channel,
							fmt.Sprintf("用户之前上传的%s已解析完成，现在可以开始针对该文件进行提问了。", typeName),
							fmt.Sprintf("📄 %s已解析完成，可以开始提问了！", typeName))
					}
					return

				case types.SummaryStatusCompleted:
					s.sendSummaryNotification(ctx, adapter, msg, knowledge, fileName, channel)
					return

				case types.SummaryStatusFailed:
					_ = s.sendSmartReply(ctx, adapter, msg, channel,
						fmt.Sprintf("用户之前上传的%s已解析完成，但摘要生成失败了。不过文件已可用于提问。", typeName),
						fmt.Sprintf("📄 %s已解析完成，可以开始提问了！（摘要生成失败）", typeName))
					return

				default:
				}

			default:
			}
		}
	}
}

func (s *Service) sendSummaryNotification(
	ctx context.Context,
	adapter Adapter,
	msg *IncomingMessage,
	knowledge *types.Knowledge,
	fileName string,
	channel *IMChannel,
) {
	summary := knowledge.Description
	if summary == "" {
		summary = knowledge.Title
	}

	typeName := fileTypeName(fileName)
	var situation, fallback string
	if summary != "" && summary != fileName {
		situation = fmt.Sprintf("用户之前上传的%s已解析完成。以下是文件的完整摘要内容：\n%s\n\n请生成一条通知消息，包含：1) 告知文件已解析完成；2) 用 Markdown 格式（标题、列表、加粗等）结构化展示上述摘要内容，不要删减或概括；3) 提示用户可以针对该文件提问。", typeName, summary)
		fallback = fmt.Sprintf("📄 %s已解析完成。\n\n**摘要：**\n\n%s\n\n---\n可以针对该文件进行提问。", typeName, summary)
	} else {
		situation = fmt.Sprintf("用户之前上传的%s已解析完成，现在可以开始针对该文件进行提问了。", typeName)
		fallback = fmt.Sprintf("📄 %s已解析完成，可以开始提问了！", typeName)
	}

	if err := s.sendSmartReply(ctx, adapter, msg, channel, situation, fallback); err != nil {
		logger.Warnf(ctx, "[IM] Failed to send summary notification: %v", err)
	}
}
