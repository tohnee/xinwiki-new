package im

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apperrors "github.com/Tencent/XinWiki/internal/errors"
	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"gorm.io/gorm"
)

func isSessionNotFound(err error) bool {
	return errors.Is(err, apperrors.ErrSessionNotFound) || errors.Is(err, gorm.ErrRecordNotFound)
}

func (s *Service) resolveSession(ctx context.Context, msg *IncomingMessage, tenantID uint64, agentID string, imChannelID string, sessionMode string) (*ChannelSession, error) {
	switch SessionMode(sessionMode) {
	case SessionModeThread:
		return s.resolveThreadSession(ctx, msg, tenantID, agentID, imChannelID)
	default:
		return s.resolveUserSession(ctx, msg, tenantID, agentID, imChannelID)
	}
}

func buildUserSessionTitle(msg *IncomingMessage) string {
	var b strings.Builder
	if msg.UserName != "" {
		b.WriteString(msg.UserName)
	} else if msg.UserID != "" {
		b.WriteString("user ")
		b.WriteString(shortID(msg.UserID))
	} else {
		b.WriteString("user")
	}
	if msg.ChatType == ChatTypeGroup && msg.ChatID != "" {
		fmt.Fprintf(&b, " · group %s", shortID(msg.ChatID))
	} else if msg.ChatType == ChatTypeDirect {
		b.WriteString(" · dm")
	}
	return b.String()
}

func buildThreadSessionTitle(msg *IncomingMessage) string {
	var b strings.Builder
	if msg.ChatID != "" {
		fmt.Fprintf(&b, "chat %s · ", shortID(msg.ChatID))
	}
	b.WriteString("thread ")
	b.WriteString(shortID(msg.ThreadID))
	return b.String()
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[len(id)-8:]
	}
	return id
}

func imInitialSessionTitle(msg *IncomingMessage, identityTitle func(*IncomingMessage) string) string {
	if strings.TrimSpace(msg.Content) != "" {
		return ""
	}
	return identityTitle(msg)
}

func (s *Service) resolveUserSession(ctx context.Context, msg *IncomingMessage, tenantID uint64, agentID string, imChannelID string) (*ChannelSession, error) {
	var cs ChannelSession
	result := s.db.Where("platform = ? AND user_id = ? AND chat_id = ? AND tenant_id = ? AND agent_id = ? AND deleted_at IS NULL",
		string(msg.Platform), msg.UserID, msg.ChatID, tenantID, agentID).
		First(&cs)

	if result.Error == nil {
		return &cs, nil
	}

	if result.Error != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("query channel session: %w", result.Error)
	}

	title := imInitialSessionTitle(msg, buildUserSessionTitle)

	newSession := &types.Session{
		TenantID:    tenantID,
		Title:       title,
		Description: fmt.Sprintf("Auto-created from %s IM integration", msg.Platform),
	}

	createdSession, err := s.sessionService.CreateSession(ctx, newSession)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	cs = ChannelSession{
		Platform:    string(msg.Platform),
		UserID:      msg.UserID,
		ChatID:      msg.ChatID,
		SessionID:   createdSession.ID,
		TenantID:    tenantID,
		AgentID:     agentID,
		IMChannelID: imChannelID,
	}
	if err := s.db.Create(&cs).Error; err != nil {
		if delErr := s.db.Where("id = ?", createdSession.ID).Delete(createdSession).Error; delErr != nil {
			logger.Warnf(ctx, "[IM] Failed to clean up orphaned session %s: %v", createdSession.ID, delErr)
		}
		var existing ChannelSession
		if findErr := s.db.Where("platform = ? AND user_id = ? AND chat_id = ? AND tenant_id = ? AND agent_id = ? AND deleted_at IS NULL",
			string(msg.Platform), msg.UserID, msg.ChatID, tenantID, agentID).
			First(&existing).Error; findErr != nil {
			return nil, fmt.Errorf("create channel session: %w (lookup fallback: %v)", err, findErr)
		}
		return &existing, nil
	}

	logger.Infof(ctx, "[IM] Created new session mapping: channel=%s/%s/%s -> session=%s",
		msg.Platform, msg.UserID, msg.ChatID, createdSession.ID)

	return &cs, nil
}

func (s *Service) resolveThreadSession(ctx context.Context, msg *IncomingMessage, tenantID uint64, agentID string, imChannelID string) (*ChannelSession, error) {
	threadID := msg.ThreadID
	if threadID == "" {
		logger.Warnf(ctx, "[IM] Thread mode but ThreadID is empty (platform=%s chat=%s), falling back to user session", msg.Platform, msg.ChatID)
		return s.resolveUserSession(ctx, msg, tenantID, agentID, imChannelID)
	}

	var cs ChannelSession
	result := s.db.Where(
		"platform = ? AND chat_id = ? AND thread_id = ? AND tenant_id = ? AND agent_id = ? AND deleted_at IS NULL",
		string(msg.Platform), msg.ChatID, threadID, tenantID, agentID,
	).First(&cs)

	if result.Error == nil {
		return &cs, nil
	}

	if result.Error != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("query thread session: %w", result.Error)
	}

	title := imInitialSessionTitle(msg, buildThreadSessionTitle)

	newSession := &types.Session{
		TenantID:    tenantID,
		Title:       title,
		Description: fmt.Sprintf("Thread-based session from %s IM", msg.Platform),
	}

	createdSession, err := s.sessionService.CreateSession(ctx, newSession)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	cs = ChannelSession{
		Platform:    string(msg.Platform),
		UserID:      msg.UserID,
		ChatID:      msg.ChatID,
		ThreadID:    threadID,
		SessionID:   createdSession.ID,
		TenantID:    tenantID,
		AgentID:     agentID,
		IMChannelID: imChannelID,
	}

	if err := s.db.Create(&cs).Error; err != nil {
		if delErr := s.db.Where("id = ?", createdSession.ID).Delete(createdSession).Error; delErr != nil {
			logger.Warnf(ctx, "[IM] Failed to clean up orphaned session %s: %v", createdSession.ID, delErr)
		}
		var existing ChannelSession
		if findErr := s.db.Where(
			"platform = ? AND chat_id = ? AND thread_id = ? AND tenant_id = ? AND agent_id = ? AND deleted_at IS NULL",
			string(msg.Platform), msg.ChatID, threadID, tenantID, agentID,
		).First(&existing).Error; findErr != nil {
			return nil, fmt.Errorf("create thread session: %w (lookup fallback: %v)", err, findErr)
		}
		return &existing, nil
	}

	logger.Infof(ctx, "[IM] Created new thread session: platform=%s thread=%s chat=%s -> session=%s",
		msg.Platform, threadID, msg.ChatID, createdSession.ID)
	return &cs, nil
}
