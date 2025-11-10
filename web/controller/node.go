// Package controller provides HTTP request handlers for node management.
package controller

import (
	"strconv"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

// NodeController handles HTTP requests related to node management.
type NodeController struct {
	BaseController
	nodeService service.NodeService
}

// NewNodeController creates a new NodeController and sets up its routes.
func NewNodeController(g *gin.RouterGroup) *NodeController {
	a := &NodeController{
		nodeService: service.NodeService{},
	}
	a.initRouter(g)
	return a
}

// initRouter initializes the routes for node-related operations.
func (a *NodeController) initRouter(g *gin.RouterGroup) {
	g.GET("/", a.getNodes)
	g.GET("/:id", a.getNode)
	g.GET("/:id/stats", a.getNodeStats)
	g.GET("/map", a.getNodesForMap)

	g.POST("/", a.addNode)
	g.POST("/:id", a.updateNode)
	g.POST("/:id/delete", a.deleteNode)
	g.POST("/:id/check", a.checkNode)
	g.POST("/:id/sync", a.syncNode)
	g.POST("/:id/detect-location", a.detectNodeLocation)
}

// getNodes retrieves all nodes.
func (a *NodeController) getNodes(c *gin.Context) {
	nodes, err := a.nodeService.GetAllNodes()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.getNodes"), err)
		return
	}
	jsonObj(c, nodes, nil)
}

// getNode retrieves a specific node by ID.
func (a *NodeController) getNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	node, err := a.nodeService.GetNode(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.getNode"), err)
		return
	}
	jsonObj(c, node, nil)
}

// addNode adds a new node.
func (a *NodeController) addNode(c *gin.Context) {
	var node model.Node
	if err := c.ShouldBindJSON(&node); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.addNode"), err)
		return
	}
	err := a.nodeService.AddNode(&node)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.addNode"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.addNodeSuccess"), nil)
}

// updateNode updates an existing node.
func (a *NodeController) updateNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	var node model.Node
	if err := c.ShouldBindJSON(&node); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.updateNode"), err)
		return
	}
	node.Id = id
	err = a.nodeService.UpdateNode(&node)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.updateNode"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.updateNodeSuccess"), nil)
}

// deleteNode deletes a node.
func (a *NodeController) deleteNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	err = a.nodeService.DeleteNode(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.deleteNode"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.deleteNodeSuccess"), nil)
}

// checkNode checks the status of a node.
func (a *NodeController) checkNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	node, err := a.nodeService.GetNode(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.getNode"), err)
		return
	}
	status, err := a.nodeService.CheckNodeStatus(node)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.checkNode"), err)
		return
	}
	jsonObj(c, gin.H{"status": status, "node": node}, nil)
}

// syncNode synchronizes statistics from a remote node.
func (a *NodeController) syncNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	err = a.nodeService.SyncNodeStats(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.syncNode"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.syncNodeSuccess"), nil)
}

// getNodeStats retrieves statistics for a node.
func (a *NodeController) getNodeStats(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	stats, err := a.nodeService.GetNodeStats(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.getNodeStats"), err)
		return
	}
	jsonObj(c, stats, nil)
}

// getNodesForMap retrieves nodes with coordinates for map display.
func (a *NodeController) getNodesForMap(c *gin.Context) {
	nodes, err := a.nodeService.GetNodesWithCoordinates()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.getNodesForMap"), err)
		return
	}
	jsonObj(c, nodes, nil)
}

// detectNodeLocation automatically detects the geographical location of a node by its IP address.
func (a *NodeController) detectNodeLocation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}

	var node *model.Node
	if id == 0 {
		// For new nodes, create a temporary node from request body
		var reqNode model.Node
		if err := c.ShouldBindJSON(&reqNode); err != nil {
			jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.detectLocation"), err)
			return
		}
		node = &reqNode
	} else {
		// For existing nodes, get from database
		var err error
		node, err = a.nodeService.GetNode(id)
		if err != nil {
			jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.getNode"), err)
			return
		}
	}

	updatedNode, err := a.nodeService.DetectNodeLocation(node)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.detectLocation"), err)
		return
	}

	// Save the updated location to database only if node exists
	if id != 0 {
		err = a.nodeService.UpdateNode(updatedNode)
		if err != nil {
			jsonMsg(c, I18nWeb(c, "pages.nodes.toasts.updateNode"), err)
			return
		}
	}

	jsonObj(c, updatedNode, nil)
}
