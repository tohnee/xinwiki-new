package im

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
)

var supportedKBFileExts = map[string]bool{
	"pdf": true, "txt": true, "docx": true, "doc": true,
	"md": true, "markdown": true,
	"png": true, "jpg": true, "jpeg": true, "gif": true,
	"csv": true, "xlsx": true, "xls": true,
	"pptx": true, "ppt": true,
}

func (s *Service) handleFileMessage(ctx context.Context, msg *IncomingMessage, adapter Adapter, channel *IMChannel) error {
	downloader, ok := adapter.(FileDownloader)
	if !ok {
		logger.Infof(ctx, "[IM] Adapter for platform %s does not support file download, ignoring file message", msg.Platform)
		return s.sendSmartReply(ctx, adapter, msg, channel,
			"用户尝试发送文件，但当前平台暂不支持文件消息处理。",
			"❌ 当前平台暂不支持文件消息处理。")
	}

	if msg.MessageType == MessageTypeImage && fileExtension(msg.FileName) == "" {
		msg.FileName = msg.FileName + ".png"
	}

	ext := fileExtension(msg.FileName)
	if ext != "" && !supportedKBFileExts[ext] {
		logger.Infof(ctx, "[IM] Unsupported file type: %s (file=%s)", ext, msg.FileName)
		return s.sendSmartReply(ctx, adapter, msg, channel,
			fmt.Sprintf("用户上传了一个不支持的文件类型「%s」。目前支持的类型包括：PDF、Word、TXT、Markdown、Excel、CSV、PPT、图片。", ext),
			fmt.Sprintf("❌ 不支持的文件类型「%s」。\n\n支持的类型：PDF、Word、TXT、Markdown、Excel、CSV、PPT、图片。", ext))
	}

	go s.processFileToKnowledgeBase(context.WithoutCancel(ctx), msg, downloader, adapter, channel)

	return nil
}

func (s *Service) processFileToKnowledgeBase(ctx context.Context, msg *IncomingMessage, downloader FileDownloader, adapter Adapter, channel *IMChannel) {
	kbID := channel.KnowledgeBaseID
	tenantID := channel.TenantID

	tenant, err := s.tenantService.GetTenantByID(ctx, tenantID)
	if err != nil {
		logger.Errorf(ctx, "[IM] Failed to get tenant %d for file processing: %v", tenantID, err)
		s.sendFileResult(ctx, adapter, msg, msg.FileName, false, "获取租户信息失败", channel)
		return
	}
	kbCtx := context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	kbCtx = context.WithValue(kbCtx, types.TenantInfoContextKey, tenant)

	reader, fileName, err := downloader.DownloadFile(ctx, msg)
	if err != nil {
		logger.Errorf(ctx, "[IM] Failed to download file from %s: %v", msg.Platform, err)
		s.sendFileResult(ctx, adapter, msg, msg.FileName, false, "下载文件失败", channel)
		return
	}
	defer reader.Close()

	logger.Debugf(ctx, "[IM] Downloaded file: original_name=%s resolved_name=%s", msg.FileName, fileName)

	ext := fileExtension(fileName)
	if !supportedKBFileExts[ext] {
		logger.Infof(ctx, "[IM] Unsupported file type after download: %s (file=%s)", ext, fileName)
		s.sendFileResult(ctx, adapter, msg, fileName, false,
			fmt.Sprintf("不支持的文件类型「%s」。支持：PDF、Word、TXT、Markdown、Excel、CSV、PPT、图片", ext), channel)
		return
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		logger.Errorf(ctx, "[IM] Failed to read file content: %v", err)
		s.sendFileResult(ctx, adapter, msg, fileName, false, "读取文件内容失败", channel)
		return
	}

	fh := newInMemoryFileHeader(fileName, content)

	knowledge, err := s.knowledgeService.CreateKnowledgeFromFile(kbCtx, kbID, fh, nil, nil, "", "", imPlatformToChannel(channel.Platform), nil)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "duplicate") || strings.Contains(errMsg, "already exists") {
			logger.Infof(ctx, "[IM] File already exists in knowledge base: %s", fileName)
			s.sendFileResult(ctx, adapter, msg, fileName, false, "文件已存在于知识库中", channel)
			return
		}
		logger.Errorf(ctx, "[IM] Failed to create knowledge from file: %v", err)
		s.sendFileResult(ctx, adapter, msg, fileName, false, "保存到知识库失败", channel)
		return
	}

	logger.Infof(ctx, "[IM] File saved to knowledge base: kb=%s knowledge=%s file=%s", kbID, knowledge.ID, fileName)
	s.sendFileResult(ctx, adapter, msg, fileName, true, "", channel)

	go s.watchAndSendSummary(ctx, kbCtx, adapter, msg, knowledge.ID, fileName, channel)
}
