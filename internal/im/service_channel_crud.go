package im

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Tencent/XinWiki/internal/logger"
	"github.com/Tencent/XinWiki/internal/types"
	"gorm.io/gorm"
)

func (s *Service) ListChannelsByAgent(agentID string, tenantID uint64) ([]IMChannel, error) {
	var channels []IMChannel
	if err := s.db.Where("agent_id = ? AND tenant_id = ? AND deleted_at IS NULL", agentID, tenantID).
		Order("created_at DESC").Find(&channels).Error; err != nil {
		return nil, err
	}
	return channels, nil
}

type ChannelWithAgent struct {
	ID          string    `json:"id"`
	TenantID    uint64    `json:"tenant_id"`
	AgentID     string    `json:"agent_id"`
	AgentName   string    `json:"agent_name"`
	Platform    string    `json:"platform"`
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Mode        string    `json:"mode"`
	OutputMode  string    `json:"output_mode"`
	SessionMode string    `json:"session_mode"`
	BotIdentity string    `json:"bot_identity"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (s *Service) ListChannelsByTenant(tenantID uint64) ([]ChannelWithAgent, error) {
	builtinIDs := types.GetBuiltinAgentIDs()
	var rows []ChannelWithAgent
	q := s.db.Table("im_channels AS c").
		Select(`c.id, c.tenant_id, c.agent_id,
                COALESCE(a.name, '') AS agent_name,
                c.platform, c.name, c.enabled, c.mode, c.output_mode,
                c.session_mode, c.bot_identity, c.created_at, c.updated_at`).
		Joins(`LEFT JOIN custom_agents AS a
               ON a.id = c.agent_id AND a.tenant_id = c.tenant_id AND a.deleted_at IS NULL`).
		Where("c.tenant_id = ? AND c.deleted_at IS NULL", tenantID)
	if len(builtinIDs) > 0 {
		q = q.Where("a.id IS NOT NULL OR c.agent_id IN ?", builtinIDs)
	} else {
		q = q.Where("a.id IS NOT NULL")
	}
	err := q.Order("c.created_at DESC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) CreateChannel(channel *IMChannel) error {
	if err := s.checkDuplicateBot(channel, ""); err != nil {
		return err
	}
	if err := s.db.Create(channel).Error; err != nil {
		return err
	}
	if channel.Enabled {
		if err := s.StartChannel(channel); err != nil {
			logger.Warnf(context.Background(), "[IM] Created channel %s but failed to start: %v", channel.ID, err)
		}
	}
	return nil
}

func (s *Service) SetChannelAgentID(ctx context.Context, channel *IMChannel, agentID string) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	agent, err := s.agentService.GetAgentByID(ctx, agentID)
	if err != nil {
		return err
	}
	if agent == nil || agent.TenantID != channel.TenantID {
		return fmt.Errorf("agent not found")
	}
	channel.AgentID = agentID
	return nil
}

func (s *Service) UpdateChannel(channel *IMChannel) error {
	if err := s.checkDuplicateBot(channel, channel.ID); err != nil {
		return err
	}
	if err := s.db.Save(channel).Error; err != nil {
		return err
	}
	s.StopChannel(channel.ID)
	if channel.Enabled {
		if err := s.StartChannel(channel); err != nil {
			logger.Warnf(context.Background(), "[IM] Updated channel %s but failed to restart: %v", channel.ID, err)
		}
	}
	return nil
}

func (s *Service) DeleteChannelsByAgent(agentID string, tenantID uint64) error {
	var channels []IMChannel
	if err := s.db.Where("agent_id = ? AND tenant_id = ? AND deleted_at IS NULL", agentID, tenantID).
		Find(&channels).Error; err != nil {
		return err
	}
	for i := range channels {
		s.StopChannel(channels[i].ID)
	}
	if len(channels) == 0 {
		return nil
	}
	return s.db.Where("agent_id = ? AND tenant_id = ? AND deleted_at IS NULL", agentID, tenantID).
		Delete(&IMChannel{}).Error
}

func (s *Service) DeleteChannel(channelID string, tenantID uint64) error {
	s.StopChannel(channelID)
	result := s.db.Where("id = ? AND tenant_id = ?", channelID, tenantID).Delete(&IMChannel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("channel not found")
	}
	return nil
}

func (s *Service) ToggleChannel(channelID string, tenantID uint64) (*IMChannel, error) {
	var ch IMChannel
	if err := s.db.Where("id = ? AND tenant_id = ? AND deleted_at IS NULL", channelID, tenantID).First(&ch).Error; err != nil {
		return nil, err
	}
	ch.Enabled = !ch.Enabled
	if err := s.db.Save(&ch).Error; err != nil {
		return nil, err
	}
	if ch.Enabled {
		if err := s.StartChannel(&ch); err != nil {
			logger.Warnf(context.Background(), "[IM] Failed to start channel %s after enable: %v", ch.ID, err)
		}
	} else {
		s.StopChannel(channelID)
	}
	return &ch, nil
}

func (s *Service) checkDuplicateBot(channel *IMChannel, excludeID string) error {
	botKey := channel.computeBotIdentity()
	if botKey == "" {
		return nil
	}

	var existing IMChannel
	query := s.db.Where("bot_identity = ? AND deleted_at IS NULL", botKey)
	if excludeID != "" {
		query = query.Where("id != ?", excludeID)
	}
	if err := query.First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("check duplicate bot: %w", err)
	}
	return fmt.Errorf("duplicate_bot: this bot is already bound to channel %q (%s); each bot can only be connected to one channel", existing.Name, existing.ID)
}
