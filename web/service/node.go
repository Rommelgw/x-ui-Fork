// Package service provides business logic for managing remote 3x-ui nodes.
package service

import (
	"encoding/json"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"

	"gorm.io/gorm"
)

// NodeService provides business logic for managing remote 3x-ui nodes.
type NodeService struct {
}

// AddNode adds a new node to the database.
func (s *NodeService) AddNode(node *model.Node) error {
	db := database.GetDB()

	if node.Name == "" {
		return common.NewError("node name is required")
	}
	if node.Host == "" {
		return common.NewError("node host is required")
	}
	if node.Port <= 0 {
		return common.NewError("node port must be greater than 0")
	}

	// Set defaults
	if node.Protocol == "" {
		node.Protocol = "https"
	}
	if node.Status == "" {
		node.Status = model.NodeStatusOffline
	}
	if node.CreatedAt == 0 {
		node.CreatedAt = time.Now().Unix()
	}
	if node.UpdatedAt == 0 {
		node.UpdatedAt = time.Now().Unix()
	}

	return db.Create(node).Error
}

// UpdateNode updates an existing node in the database.
func (s *NodeService) UpdateNode(node *model.Node) error {
	db := database.GetDB()

	if node.Id <= 0 {
		return common.NewError("node ID is required")
	}
	if node.Name == "" {
		return common.NewError("node name is required")
	}
	if node.Host == "" {
		return common.NewError("node host is required")
	}
	if node.Port <= 0 {
		return common.NewError("node port must be greater than 0")
	}

	node.UpdatedAt = time.Now().Unix()

	return db.Save(node).Error
}

// DeleteNode deletes a node from the database.
func (s *NodeService) DeleteNode(id int) error {
	db := database.GetDB()

	// Also delete associated stats
	db.Where("node_id = ?", id).Delete(&model.NodeStats{})

	return db.Delete(&model.Node{}, id).Error
}

// GetNode retrieves a node by ID.
func (s *NodeService) GetNode(id int) (*model.Node, error) {
	db := database.GetDB()
	var node model.Node
	err := db.First(&node, id).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// GetAllNodes retrieves all nodes from the database.
func (s *NodeService) GetAllNodes() ([]*model.Node, error) {
	db := database.GetDB()
	var nodes []*model.Node
	err := db.Find(&nodes).Error
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// GetEnabledNodes retrieves all enabled nodes.
func (s *NodeService) GetEnabledNodes() ([]*model.Node, error) {
	db := database.GetDB()
	var nodes []*model.Node
	err := db.Where("enable = ?", true).Find(&nodes).Error
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// CheckNodeStatus checks the status of a node and updates it in the database.
func (s *NodeService) CheckNodeStatus(node *model.Node) (string, error) {
	client := NewNodeClient(node)
	status, err := client.CheckConnection()

	if err != nil {
		node.Status = model.NodeStatusOffline
	} else {
		node.Status = model.NodeStatus(status)
	}
	node.LastCheck = time.Now().Unix()

	// Update node status in database
	db := database.GetDB()
	db.Model(node).Updates(map[string]interface{}{
		"status":    node.Status,
		"lastCheck": node.LastCheck,
	})

	return status, err
}

// SyncNodeStats synchronizes statistics from a remote node.
func (s *NodeService) SyncNodeStats(nodeId int) error {
	node, err := s.GetNode(nodeId)
	if err != nil {
		return err
	}

	client := NewNodeClient(node)
	status, err := client.GetStatus()
	if err != nil {
		return common.NewErrorf("failed to get node status: %v", err)
	}

	// Get inbounds count
	inbounds, err := client.GetInbounds()
	if err != nil {
		logger.Warningf("Failed to get inbounds from node %d: %v", nodeId, err)
		inbounds = []*model.Inbound{}
	}

	// Count clients
	clientCount := 0
	for _, inbound := range inbounds {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}
		clients, ok := settings["clients"].([]interface{})
		if ok {
			clientCount += len(clients)
		}
	}

	// Create or update node stats
	db := database.GetDB()
	nodeStats := &model.NodeStats{
		NodeId:      nodeId,
		Cpu:         status.Cpu,
		Mem:         status.Mem.Current,
		MemTotal:    status.Mem.Total,
		Disk:        status.Disk.Current,
		DiskTotal:   status.Disk.Total,
		NetUp:       status.NetIO.Up,
		NetDown:     status.NetIO.Down,
		Uptime:      status.Uptime,
		XrayStatus:  string(status.Xray.State),
		Clients:     clientCount,
		Inbounds:    len(inbounds),
		CollectedAt: time.Now().Unix(),
	}

	// Check if stats already exist for this node
	var existingStats model.NodeStats
	err = db.Where("node_id = ?", nodeId).Order("collected_at DESC").First(&existingStats).Error
	if err == gorm.ErrRecordNotFound {
		// Create new stats record
		return db.Create(nodeStats).Error
	} else if err != nil {
		return err
	}

	// Update existing stats
	existingStats.Cpu = nodeStats.Cpu
	existingStats.Mem = nodeStats.Mem
	existingStats.MemTotal = nodeStats.MemTotal
	existingStats.Disk = nodeStats.Disk
	existingStats.DiskTotal = nodeStats.DiskTotal
	existingStats.NetUp = nodeStats.NetUp
	existingStats.NetDown = nodeStats.NetDown
	existingStats.Uptime = nodeStats.Uptime
	existingStats.XrayStatus = nodeStats.XrayStatus
	existingStats.Clients = nodeStats.Clients
	existingStats.Inbounds = nodeStats.Inbounds
	existingStats.CollectedAt = nodeStats.CollectedAt

	return db.Save(&existingStats).Error
}

// GetNodeStats retrieves the latest statistics for a node.
func (s *NodeService) GetNodeStats(nodeId int) (*model.NodeStats, error) {
	db := database.GetDB()
	var stats model.NodeStats
	err := db.Where("node_id = ?", nodeId).Order("collected_at DESC").First(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetAllNodesStats retrieves the latest statistics for all nodes.
func (s *NodeService) GetAllNodesStats() ([]*model.NodeStats, error) {
	db := database.GetDB()
	var stats []*model.NodeStats

	// Get the latest stats for each node
	err := db.Raw(`
		SELECT * FROM node_stats
		WHERE id IN (
			SELECT MAX(id) FROM node_stats
			GROUP BY node_id
		)
	`).Scan(&stats).Error

	if err != nil {
		return nil, err
	}
	return stats, nil
}

// GetNodesWithCoordinates retrieves all nodes that have coordinates for map display.
func (s *NodeService) GetNodesWithCoordinates() ([]*model.Node, error) {
	db := database.GetDB()
	var nodes []*model.Node
	err := db.Where("latitude != 0 AND longitude != 0").Find(&nodes).Error
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// DetectNodeLocation automatically detects the geographical location of a node by its IP address.
func (s *NodeService) DetectNodeLocation(node *model.Node) (*model.Node, error) {
	if node.Host == "" {
		return nil, common.NewError("node host is required for location detection")
	}

	geoService := NewGeolocationService()
	location, err := geoService.GetLocationByIP(node.Host)
	if err != nil {
		logger.Warningf("Failed to detect location for node %s (%s): %v", node.Name, node.Host, err)
		return nil, common.NewErrorf("failed to detect location: %v", err)
	}

	// Update node with detected location
	node.Country = location.Country
	node.City = location.City
	node.Location = location.Location
	node.Latitude = location.Latitude
	node.Longitude = location.Longitude

	logger.Infof("Detected location for node %s (%s): %s, %s (%.4f, %.4f)",
		node.Name, node.Host, location.City, location.Country, location.Latitude, location.Longitude)

	return node, nil
}
