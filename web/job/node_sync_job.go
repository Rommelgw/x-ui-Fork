package job

import (
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// NodeSyncJob periodically synchronizes statistics from all enabled nodes.
type NodeSyncJob struct {
}

func NewNodeSyncJob() *NodeSyncJob { return &NodeSyncJob{} }

func (j *NodeSyncJob) Run() {
	dashboard := service.DashboardService{}
	if err := dashboard.SyncAllNodesStats(); err != nil {
		logger.Debugf("NodeSyncJob: sync error: %v", err)
	}
}
