package service

import (
	"context"
	"errors"
	"time"

	"x-ui/internal/model"

	"database/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NodeService struct {
	db *gorm.DB
}

func NewNodeService(db *gorm.DB) *NodeService {
	return &NodeService{db: db}
}

func (s *NodeService) UpsertNode(ctx context.Context, node *model.Node) error {
	if node.ID == "" {
		return errors.New("node ID is required")
	}

	node.LastSeen = time.Now()

	return s.db.WithContext(ctx).Clauses(
		clauseOnConflictUpdate(),
	).Create(node).Error
}

func (s *NodeService) GetNode(ctx context.Context, nodeID string) (*model.Node, error) {
	var node model.Node
	if err := s.db.WithContext(ctx).Where("id = ?", nodeID).First(&node).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

func (s *NodeService) UpdateNodeStatus(ctx context.Context, nodeID string, status model.NodeStatus) error {
	return s.db.WithContext(ctx).Model(&model.Node{}).Where("id = ?", nodeID).
		Updates(map[string]interface{}{
			"status":    status,
			"last_seen": time.Now(),
		}).Error
}

func (s *NodeService) SaveNodeStats(ctx context.Context, nodeID string, stats *model.ClientNodeStat) error {
	stats.NodeID = nodeID
	return s.db.WithContext(ctx).Where("client_id = ? AND node_id = ?", stats.ClientID, nodeID).
		Assign(map[string]interface{}{
			"upload":    stats.Upload,
			"download":  stats.Download,
			"last_used": stats.LastUsed,
		}).FirstOrCreate(stats).Error
}

func (s *NodeService) GetAllNodes(ctx context.Context) ([]*model.Node, error) {
	var nodes []*model.Node
	if err := s.db.WithContext(ctx).Preload("Groups").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

type DashboardMetrics struct {
	TotalNodes    int64     `json:"total_nodes"`
	OnlineNodes   int64     `json:"online_nodes"`
	DegradedNodes int64     `json:"degraded_nodes"`
	OfflineNodes  int64     `json:"offline_nodes"`
	OnlineUsers   int64     `json:"online_users"`
	Traffic24hGB  float64   `json:"traffic_24h_gb"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (s *NodeService) GetDashboardMetrics(ctx context.Context) (*DashboardMetrics, error) {
	metrics := &DashboardMetrics{UpdatedAt: time.Now()}

	if err := s.db.WithContext(ctx).Model(&model.Node{}).Count(&metrics.TotalNodes).Error; err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).
		Model(&model.Node{}).
		Where("status IN ?", []model.NodeStatus{model.NodeStatusOnline, model.NodeStatusSyncing}).
		Count(&metrics.OnlineNodes).Error; err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).
		Model(&model.Node{}).
		Where("status = ?", model.NodeStatusDegraded).
		Count(&metrics.DegradedNodes).Error; err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).
		Model(&model.Node{}).
		Where("status = ?", model.NodeStatusOffline).
		Count(&metrics.OfflineNodes).Error; err != nil {
		return nil, err
	}

	threshold := time.Now().Add(-10 * time.Minute)
	if err := s.db.WithContext(ctx).
		Model(&model.ClientNodeStat{}).
		Where("last_used IS NOT NULL AND last_used > ?", threshold).
		Distinct("client_id").
		Count(&metrics.OnlineUsers).Error; err != nil {
		return nil, err
	}

	var trafficBytes sql.NullFloat64
	if err := s.db.WithContext(ctx).
		Model(&model.ClientNodeStat{}).
		Select("COALESCE(SUM(upload + download), 0)").
		Where("updated_at > ?", time.Now().Add(-24*time.Hour)).
		Scan(&trafficBytes).Error; err != nil {
		return nil, err
	}
	if trafficBytes.Valid {
		metrics.Traffic24hGB = trafficBytes.Float64 / (1024 * 1024 * 1024)
	}

	return metrics, nil
}

// clauseOnConflictUpdate builds a reusable on conflict clause.
func clauseOnConflictUpdate() clause.OnConflict {
	return clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "master_url", "secret_key", "status", "ip_address", "hostname", "location", "xray_version", "listen_addr", "last_seen"}),
	}
}
