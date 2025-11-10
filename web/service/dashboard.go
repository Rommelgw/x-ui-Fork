// Package service provides business logic for dashboard aggregation.
package service

import (
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
)

// AggregatedStats represents aggregated statistics from all nodes.
type AggregatedStats struct {
	TotalNodes    int     `json:"totalNodes"`
	OnlineNodes   int     `json:"onlineNodes"`
	OfflineNodes  int     `json:"offlineNodes"`
	ErrorNodes    int     `json:"errorNodes"`
	TotalClients  int     `json:"totalClients"`
	TotalInbounds int     `json:"totalInbounds"`
	TotalCpu      float64 `json:"totalCpu"`
	AvgCpu        float64 `json:"avgCpu"`
	TotalMem      uint64  `json:"totalMem"`
	TotalMemUsed  uint64  `json:"totalMemUsed"`
	TotalDisk     uint64  `json:"totalDisk"`
	TotalDiskUsed uint64  `json:"totalDiskUsed"`
	TotalNetUp    uint64  `json:"totalNetUp"`
	TotalNetDown  uint64  `json:"totalNetDown"`
	TotalUptime   uint64  `json:"totalUptime"`
	AvgUptime     uint64  `json:"avgUptime"`
	XrayRunning   int     `json:"xrayRunning"`
	XrayStopped   int     `json:"xrayStopped"`
	XrayError     int     `json:"xrayError"`
}

// DashboardData represents complete dashboard data including nodes and aggregated stats.
type DashboardData struct {
	AggregatedStats AggregatedStats    `json:"aggregatedStats"`
	Nodes           []*model.Node      `json:"nodes"`
	NodesStats      []*model.NodeStats `json:"nodesStats"`
	LastUpdate      int64              `json:"lastUpdate"`
}

// DashboardService provides business logic for dashboard data aggregation.
type DashboardService struct {
	nodeService NodeService
}

// GetAggregatedStats retrieves aggregated statistics from all nodes.
func (s *DashboardService) GetAggregatedStats() (*AggregatedStats, error) {
	nodeService := NodeService{}
	nodes, err := nodeService.GetAllNodes()
	if err != nil {
		return nil, err
	}

	stats, err := nodeService.GetAllNodesStats()
	if err != nil {
		// If no stats available, return basic stats from nodes
		return s.getBasicAggregatedStats(nodes), nil
	}

	aggregated := &AggregatedStats{
		TotalNodes: len(nodes),
	}

	// Create a map of node stats by node ID for quick lookup
	statsMap := make(map[int]*model.NodeStats)
	for _, stat := range stats {
		statsMap[stat.NodeId] = stat
	}

	// Aggregate statistics
	for _, node := range nodes {
		// Count nodes by status
		switch node.Status {
		case model.NodeStatusOnline:
			aggregated.OnlineNodes++
		case model.NodeStatusOffline:
			aggregated.OfflineNodes++
		case model.NodeStatusError:
			aggregated.ErrorNodes++
		}

		// Get stats for this node
		nodeStats, exists := statsMap[node.Id]
		if !exists {
			continue
		}

		// Aggregate statistics
		aggregated.TotalClients += nodeStats.Clients
		aggregated.TotalInbounds += nodeStats.Inbounds
		aggregated.TotalCpu += nodeStats.Cpu
		aggregated.TotalMem += nodeStats.MemTotal
		aggregated.TotalMemUsed += nodeStats.Mem
		aggregated.TotalDisk += nodeStats.DiskTotal
		aggregated.TotalDiskUsed += nodeStats.Disk
		aggregated.TotalNetUp += nodeStats.NetUp
		aggregated.TotalNetDown += nodeStats.NetDown
		aggregated.TotalUptime += nodeStats.Uptime

		// Count Xray status
		switch nodeStats.XrayStatus {
		case "running":
			aggregated.XrayRunning++
		case "stop":
			aggregated.XrayStopped++
		case "error":
			aggregated.XrayError++
		}
	}

	// Calculate averages
	if aggregated.TotalNodes > 0 {
		aggregated.AvgCpu = aggregated.TotalCpu / float64(aggregated.TotalNodes)
		aggregated.AvgUptime = aggregated.TotalUptime / uint64(aggregated.TotalNodes)
	}

	return aggregated, nil
}

// getBasicAggregatedStats returns basic aggregated stats from nodes without detailed stats.
func (s *DashboardService) getBasicAggregatedStats(nodes []*model.Node) *AggregatedStats {
	aggregated := &AggregatedStats{
		TotalNodes: len(nodes),
	}

	for _, node := range nodes {
		switch node.Status {
		case model.NodeStatusOnline:
			aggregated.OnlineNodes++
		case model.NodeStatusOffline:
			aggregated.OfflineNodes++
		case model.NodeStatusError:
			aggregated.ErrorNodes++
		}
	}

	return aggregated
}

// GetDashboardData retrieves complete dashboard data including nodes and aggregated stats.
func (s *DashboardService) GetDashboardData() (*DashboardData, error) {
	nodeService := NodeService{}
	nodes, err := nodeService.GetAllNodes()
	if err != nil {
		return nil, err
	}

	stats, err := nodeService.GetAllNodesStats()
	if err != nil {
		stats = []*model.NodeStats{}
	}

	aggregatedStats, err := s.GetAggregatedStats()
	if err != nil {
		return nil, err
	}

	return &DashboardData{
		AggregatedStats: *aggregatedStats,
		Nodes:           nodes,
		NodesStats:      stats,
		LastUpdate:      time.Now().Unix(),
	}, nil
}

// GetNodesForMap retrieves nodes with coordinates for map display.
func (s *DashboardService) GetNodesForMap() ([]*model.Node, error) {
	nodeService := NodeService{}
	return nodeService.GetNodesWithCoordinates()
}

// SyncAllNodesStats synchronizes statistics from all enabled nodes.
func (s *DashboardService) SyncAllNodesStats() error {
	nodeService := NodeService{}
	nodes, err := nodeService.GetEnabledNodes()
	if err != nil {
		return err
	}

	var lastError error
	for _, node := range nodes {
		err := nodeService.SyncNodeStats(node.Id)
		if err != nil {
			lastError = err
			// Continue syncing other nodes even if one fails
			continue
		}
	}

	return lastError
}

// CheckAllNodesStatus checks the status of all enabled nodes.
func (s *DashboardService) CheckAllNodesStatus() error {
	nodeService := NodeService{}
	nodes, err := nodeService.GetEnabledNodes()
	if err != nil {
		return err
	}

	var lastError error
	for _, node := range nodes {
		_, err := nodeService.CheckNodeStatus(node)
		if err != nil {
			lastError = err
			// Continue checking other nodes even if one fails
			continue
		}
	}

	return lastError
}
