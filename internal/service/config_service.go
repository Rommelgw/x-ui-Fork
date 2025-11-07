package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"x-ui/internal/model"

	"gorm.io/gorm"
)

type NodeConfig struct {
	Inbounds    []map[string]any
	Outbounds   []map[string]any
	Clients     []map[string]any
	Routing     map[string]any
	DNS         map[string]any
	Policy      map[string]any
	Transport   map[string]any
	Log         map[string]any
	GeneratedAt time.Time
}

type ConfigService struct {
	db *gorm.DB
}

func NewConfigService(db *gorm.DB) *ConfigService {
	return &ConfigService{db: db}
}

func (s *ConfigService) GetNodeConfiguration(ctx context.Context, nodeID string) (*NodeConfig, error) {
	if nodeID == "" {
		return nil, errors.New("node id is required")
	}

	var node model.Node
	if err := s.db.WithContext(ctx).Preload("Groups").Where("id = ?", nodeID).First(&node).Error; err != nil {
		return nil, err
	}

	groupIDs := make([]uint, 0, len(node.Groups))
	for _, g := range node.Groups {
		groupIDs = append(groupIDs, g.ID)
	}

	inbounds, err := s.loadCentralInbounds(ctx)
	if err != nil {
		return nil, err
	}

	clients, err := s.loadClientsForGroups(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	cfg := &NodeConfig{
		Inbounds:    inbounds,
		Outbounds:   defaultOutbounds(),
		Clients:     clients,
		GeneratedAt: time.Now(),
		Log: map[string]any{
			"loglevel": "info",
		},
	}

	return cfg, nil
}

func (s *ConfigService) loadCentralInbounds(ctx context.Context) ([]map[string]any, error) {
	var inbounds []model.CentralInbound
	if err := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Find(&inbounds).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(inbounds))
	for _, inbound := range inbounds {
		cfg := map[string]any{
			"tag":      inbound.Name,
			"protocol": inbound.Protocol,
			"port":     inbound.Port,
		}

		settings := map[string]any{}
		if len(inbound.Settings) > 0 {
			if err := json.Unmarshal(inbound.Settings, &settings); err != nil {
				return nil, fmt.Errorf("inbound %s settings: %w", inbound.Name, err)
			}
		}
		cfg["settings"] = settings

		if len(inbound.StreamSettings) > 0 {
			var stream map[string]any
			if err := json.Unmarshal(inbound.StreamSettings, &stream); err != nil {
				return nil, fmt.Errorf("inbound %s stream settings: %w", inbound.Name, err)
			}
			cfg["streamSettings"] = stream
		}

		if len(inbound.Sniffing) > 0 {
			var sniff map[string]any
			if err := json.Unmarshal(inbound.Sniffing, &sniff); err != nil {
				return nil, fmt.Errorf("inbound %s sniffing: %w", inbound.Name, err)
			}
			cfg["sniffing"] = sniff
		}

		if inbound.ClientStats {
			cfg["allocate"] = map[string]any{"strategy": "always"}
		}

		result = append(result, cfg)
	}

	return result, nil
}

func (s *ConfigService) loadClientsForGroups(ctx context.Context, groupIDs []uint) ([]map[string]any, error) {
	if len(groupIDs) == 0 {
		return nil, nil
	}

	var subs []model.UserSubscription
	if err := s.db.WithContext(ctx).
		Where("group_id IN ?", groupIDs).
		Where("is_active = ?", true).
		Find(&subs).Error; err != nil {
		return nil, err
	}

	clientIDs := make([]uint, 0, len(subs))
	subMap := make(map[uint]model.UserSubscription)
	for _, sub := range subs {
		if sub.ExpireAt != nil && sub.ExpireAt.Before(time.Now()) {
			continue
		}
		clientIDs = append(clientIDs, sub.UserID)
		subMap[sub.UserID] = sub
	}

	if len(clientIDs) == 0 {
		return nil, nil
	}

	var clients []model.Client
	if err := s.db.WithContext(ctx).
		Where("id IN ?", clientIDs).
		Where("is_active = ?", true).
		Find(&clients).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(clients))
	now := time.Now()
	for _, client := range clients {
		if client.ExpireAt != nil && client.ExpireAt.Before(now) {
			continue
		}
		sub := subMap[client.ID]
		if sub.TrafficLimit > 0 && sub.UsedTraffic >= sub.TrafficLimit {
			continue
		}

		entry := map[string]any{
			"id":             client.UUID,
			"email":          client.Email,
			"subscriptionId": sub.ID,
			"trafficLimit":   sub.TrafficLimit,
			"usedTraffic":    sub.UsedTraffic,
		}

		if client.SubscriptionID != "" {
			entry["subscriptionRef"] = client.SubscriptionID
		}
		if client.TrafficLimit > 0 {
			entry["globalTrafficLimit"] = client.TrafficLimit
			entry["globalUsedTraffic"] = client.UsedTraffic
		}
		if client.ExpireAt != nil {
			entry["expireAt"] = client.ExpireAt
		}

		result = append(result, entry)
	}

	return result, nil
}

func defaultOutbounds() []map[string]any {
	return []map[string]any{
		{
			"tag":      "direct",
			"protocol": "freedom",
			"settings": map[string]any{},
		},
		{
			"tag":      "blocked",
			"protocol": "blackhole",
			"settings": map[string]any{},
		},
	}
}
