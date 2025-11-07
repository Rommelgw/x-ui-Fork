package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"x-ui/internal/model"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type SubscriptionBundle struct {
	JSON         string    `json:"json"`
	Clash        string    `json:"clash"`
	V2Ray        string    `json:"v2ray"`
	Shadowrocket string    `json:"shadowrocket"`
	GeneratedAt  time.Time `json:"generated_at"`
}

type SubscriptionService struct {
	db            *gorm.DB
	configService *ConfigService
}

func NewSubscriptionService(db *gorm.DB, configService *ConfigService) *SubscriptionService {
	return &SubscriptionService{db: db, configService: configService}
}

func (s *SubscriptionService) GenerateBundle(ctx context.Context, clientUUID string) (*SubscriptionBundle, error) {
	var client model.Client
	if err := s.db.WithContext(ctx).Where("uuid = ?", clientUUID).First(&client).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	if !client.IsActive {
		return nil, fmt.Errorf("client %s is inactive", clientUUID)
	}
	if client.ExpireAt != nil && client.ExpireAt.Before(now) {
		return nil, fmt.Errorf("client %s subscription expired", clientUUID)
	}

	var subs []model.UserSubscription
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", client.ID).
		Where("is_active = ?", true).
		Find(&subs).Error; err != nil {
		return nil, err
	}

	groupIDs := make([]uint, 0, len(subs))
	for _, sub := range subs {
		if sub.ExpireAt != nil && sub.ExpireAt.Before(now) {
			continue
		}
		groupIDs = append(groupIDs, sub.GroupID)
	}

	if len(groupIDs) == 0 {
		return nil, fmt.Errorf("no active subscriptions for client %s", clientUUID)
	}

	inbounds, err := s.configService.loadCentralInbounds(ctx)
	if err != nil {
		return nil, err
	}

	endpoints, err := s.buildEndpoints(ctx, groupIDs, inbounds)
	if err != nil {
		return nil, err
	}

	jsonPayload, err := s.buildJSONBundle(client, endpoints)
	if err != nil {
		return nil, err
	}

	clashPayload, err := s.buildClashBundle(client, endpoints)
	if err != nil {
		return nil, err
	}

	v2rayLinks := make([]string, 0, len(endpoints))
	shadowLinks := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		link := s.generateLink(client, ep)
		if link == "" {
			continue
		}
		v2rayLinks = append(v2rayLinks, link)
		shadowLinks = append(shadowLinks, link)
	}

	bundle := &SubscriptionBundle{
		JSON:         jsonPayload,
		Clash:        clashPayload,
		V2Ray:        strings.Join(v2rayLinks, "\n"),
		Shadowrocket: strings.Join(shadowLinks, "\n"),
		GeneratedAt:  time.Now(),
	}

	return bundle, nil
}

type endpoint struct {
	Name           string
	Protocol       string
	Host           string
	Port           int
	Weight         int
	Settings       map[string]any
	StreamSettings map[string]any
}

func (s *SubscriptionService) buildEndpoints(ctx context.Context, groupIDs []uint, inbounds []map[string]any) ([]endpoint, error) {
	var endpoints []endpoint
	for _, groupID := range groupIDs {
		nodes, err := s.loadGroupNodes(ctx, groupID)
		if err != nil {
			return nil, err
		}

		for _, node := range nodes {
			host := node.Hostname
			if host == "" {
				host = node.IPAddress
			}
			for _, inbound := range inbounds {
				ep := endpoint{
					Name:           fmt.Sprintf("%s-%s", safeString(inbound["tag"]), node.Name),
					Protocol:       safeString(inbound["protocol"]),
					Host:           host,
					Port:           safeInt(inbound["port"], 0),
					Weight:         node.Weight,
					Settings:       cloneAnyMap(inbound["settings"]),
					StreamSettings: cloneAnyMap(inbound["streamSettings"]),
				}
				endpoints = append(endpoints, ep)
			}
		}
	}

	sort.SliceStable(endpoints, func(i, j int) bool {
		if endpoints[i].Weight == endpoints[j].Weight {
			return endpoints[i].Name < endpoints[j].Name
		}
		return endpoints[i].Weight > endpoints[j].Weight
	})

	return endpoints, nil
}

type nodeRecord struct {
	ID        string
	Name      string
	Hostname  string
	IPAddress string
	Status    model.NodeStatus
	Weight    int
}

func (s *SubscriptionService) loadGroupNodes(ctx context.Context, groupID uint) ([]nodeRecord, error) {
	var records []struct {
		NodeID   string
		Weight   int
		IsActive bool
	}

	if err := s.db.WithContext(ctx).
		Table("group_nodes").
		Select("node_id, weight, is_active").
		Where("group_id = ?", groupID).
		Find(&records).Error; err != nil {
		return nil, err
	}

	nodeIDs := make([]string, 0, len(records))
	weights := make(map[string]int)
	for _, rec := range records {
		if !rec.IsActive {
			continue
		}
		nodeIDs = append(nodeIDs, rec.NodeID)
		weights[rec.NodeID] = rec.Weight
	}

	if len(nodeIDs) == 0 {
		return nil, nil
	}

	var nodes []model.Node
	if err := s.db.WithContext(ctx).
		Where("id IN ?", nodeIDs).
		Where("status <> ?", model.NodeStatusOffline).
		Find(&nodes).Error; err != nil {
		return nil, err
	}

	result := make([]nodeRecord, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, nodeRecord{
			ID:        node.ID,
			Name:      node.Name,
			Hostname:  node.Hostname,
			IPAddress: node.IPAddress,
			Status:    node.Status,
			Weight:    weights[node.ID],
		})
	}

	return result, nil
}

func (s *SubscriptionService) buildJSONBundle(client model.Client, endpoints []endpoint) (string, error) {
	payload := map[string]any{
		"client": map[string]any{
			"email": client.Email,
			"uuid":  client.UUID,
		},
		"endpoints": make([]map[string]any, 0, len(endpoints)),
	}

	for _, ep := range endpoints {
		payload["endpoints"] = append(payload["endpoints"].([]map[string]any), map[string]any{
			"name":            ep.Name,
			"protocol":        ep.Protocol,
			"host":            ep.Host,
			"port":            ep.Port,
			"weight":          ep.Weight,
			"settings":        ep.Settings,
			"stream_settings": ep.StreamSettings,
		})
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *SubscriptionService) buildClashBundle(client model.Client, endpoints []endpoint) (string, error) {
	type clashConfig struct {
		Proxies     []map[string]any `yaml:"proxies"`
		ProxyGroups []map[string]any `yaml:"proxy-groups"`
	}

	cfg := clashConfig{
		Proxies:     make([]map[string]any, 0, len(endpoints)),
		ProxyGroups: []map[string]any{},
	}

	allNames := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		proxy := map[string]any{
			"name":             ep.Name,
			"server":           ep.Host,
			"port":             ep.Port,
			"type":             protocolToClashType(ep.Protocol),
			"uuid":             client.UUID,
			"cipher":           "auto",
			"tls":              false,
			"skip-cert-verify": true,
		}

		if network := extractNetwork(ep.StreamSettings); network != "" {
			proxy["network"] = network
		}
		if tlsEnabled(ep.StreamSettings) {
			proxy["tls"] = true
		}
		if path := extractWSPath(ep.StreamSettings); path != "" {
			proxy["ws-path"] = path
		}
		if host := extractWSHost(ep.StreamSettings); host != "" {
			proxy["ws-headers"] = map[string]string{"Host": host}
		}

		cfg.Proxies = append(cfg.Proxies, proxy)
		allNames = append(allNames, ep.Name)
	}

	if len(allNames) > 0 {
		cfg.ProxyGroups = append(cfg.ProxyGroups, map[string]any{
			"name":     "VPN",
			"type":     "load-balance",
			"strategy": "round-robin",
			"proxies":  allNames,
		})
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *SubscriptionService) generateLink(client model.Client, ep endpoint) string {
	switch ep.Protocol {
	case "vmess":
		cfg := map[string]any{
			"v":    "2",
			"ps":   ep.Name,
			"add":  ep.Host,
			"port": fmt.Sprintf("%d", ep.Port),
			"id":   client.UUID,
			"aid":  "0",
			"type": "none",
		}
		if network := extractNetwork(ep.StreamSettings); network != "" {
			cfg["net"] = network
		}
		if tlsEnabled(ep.StreamSettings) {
			cfg["tls"] = "tls"
		}
		data, _ := json.Marshal(cfg)
		return "vmess://" + base64.StdEncoding.EncodeToString(data)
	case "vless":
		params := url.Values{}
		if network := extractNetwork(ep.StreamSettings); network != "" {
			params.Set("type", network)
		}
		if tlsEnabled(ep.StreamSettings) {
			params.Set("security", "tls")
		}
		if path := extractWSPath(ep.StreamSettings); path != "" {
			params.Set("path", path)
		}
		if host := extractWSHost(ep.StreamSettings); host != "" {
			params.Set("host", host)
		}
		u := url.URL{
			Scheme:   "vless",
			Host:     fmt.Sprintf("%s:%d", ep.Host, ep.Port),
			RawQuery: params.Encode(),
			Fragment: url.QueryEscape(ep.Name),
		}
		u.User = url.User(client.UUID)
		return u.String()
	case "trojan":
		params := url.Values{}
		if host := extractWSHost(ep.StreamSettings); host != "" {
			params.Set("sni", host)
		}
		u := url.URL{
			Scheme:   "trojan",
			User:     url.UserPassword(client.UUID, ""),
			Host:     fmt.Sprintf("%s:%d", ep.Host, ep.Port),
			RawQuery: params.Encode(),
			Fragment: url.QueryEscape(ep.Name),
		}
		return u.String()
	case "shadowsocks":
		method := safeString(ep.Settings["method"])
		password := client.UUID
		if pwd := safeString(ep.Settings["password"]); pwd != "" {
			password = pwd
		}
		raw := fmt.Sprintf("%s:%s@%s:%d", method, password, ep.Host, ep.Port)
		return "ss://" + base64.StdEncoding.EncodeToString([]byte(raw)) + "#" + url.QueryEscape(ep.Name)
	default:
		return ""
	}
}

func protocolToClashType(protocol string) string {
	switch protocol {
	case "vmess":
		return "vmess"
	case "vless":
		return "vless"
	case "trojan":
		return "trojan"
	case "shadowsocks":
		return "ss"
	default:
		return protocol
	}
}

func extractNetwork(stream map[string]any) string {
	if stream == nil {
		return ""
	}
	if v, ok := stream["network"].(string); ok {
		return v
	}
	return ""
}

func tlsEnabled(stream map[string]any) bool {
	if stream == nil {
		return false
	}
	if v, ok := stream["security"].(string); ok {
		return strings.EqualFold(v, "tls") || strings.EqualFold(v, "reality")
	}
	return false
}

func extractWSPath(stream map[string]any) string {
	if stream == nil {
		return ""
	}
	if ws, ok := stream["wsSettings"].(map[string]any); ok {
		if path, ok := ws["path"].(string); ok {
			return path
		}
	}
	return ""
}

func extractWSHost(stream map[string]any) string {
	if stream == nil {
		return ""
	}
	if ws, ok := stream["wsSettings"].(map[string]any); ok {
		if headers, ok := ws["headers"].(map[string]any); ok {
			if host, ok := headers["Host"].(string); ok {
				return host
			}
		}
	}
	return ""
}

func safeString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func safeInt(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return fallback
	}
}

func cloneAnyMap(value any) map[string]any {
	src, ok := value.(map[string]any)
	if !ok || src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
