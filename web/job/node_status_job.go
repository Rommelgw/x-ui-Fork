package job

import (
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// NodeStatusJob periodically checks status of all enabled nodes.
type NodeStatusJob struct {
}

func NewNodeStatusJob() *NodeStatusJob { return &NodeStatusJob{} }

func (j *NodeStatusJob) Run() {
	dashboard := service.DashboardService{}
	if err := dashboard.CheckAllNodesStatus(); err != nil {
		logger.Debugf("NodeStatusJob: check error: %v", err)
	}
}
