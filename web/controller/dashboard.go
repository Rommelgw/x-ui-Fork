// Package controller provides HTTP request handlers for dashboard management.
package controller

import (
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

// DashboardController handles HTTP requests related to dashboard data.
type DashboardController struct {
	BaseController
	dashboardService service.DashboardService
}

// NewDashboardController creates a new DashboardController and sets up its routes.
func NewDashboardController(g *gin.RouterGroup) *DashboardController {
	a := &DashboardController{
		dashboardService: service.DashboardService{},
	}
	a.initRouter(g)
	return a
}

// initRouter initializes the routes for dashboard-related operations.
func (a *DashboardController) initRouter(g *gin.RouterGroup) {
	g.GET("/stats", a.getAggregatedStats)
	g.GET("/data", a.getDashboardData)
	g.GET("/map", a.getNodesForMap)
	g.POST("/sync", a.syncAllNodesStats)
	g.POST("/check", a.checkAllNodesStatus)
}

// getAggregatedStats retrieves aggregated statistics from all nodes.
func (a *DashboardController) getAggregatedStats(c *gin.Context) {
	stats, err := a.dashboardService.GetAggregatedStats()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.dashboard.toasts.getStats"), err)
		return
	}
	jsonObj(c, stats, nil)
}

// getDashboardData retrieves complete dashboard data.
func (a *DashboardController) getDashboardData(c *gin.Context) {
	data, err := a.dashboardService.GetDashboardData()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.dashboard.toasts.getData"), err)
		return
	}
	jsonObj(c, data, nil)
}

// getNodesForMap retrieves nodes with coordinates for map display.
func (a *DashboardController) getNodesForMap(c *gin.Context) {
	nodes, err := a.dashboardService.GetNodesForMap()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.dashboard.toasts.getNodesForMap"), err)
		return
	}
	jsonObj(c, nodes, nil)
}

// syncAllNodesStats synchronizes statistics from all enabled nodes.
func (a *DashboardController) syncAllNodesStats(c *gin.Context) {
	err := a.dashboardService.SyncAllNodesStats()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.dashboard.toasts.syncStats"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.dashboard.toasts.syncStatsSuccess"), nil)
}

// checkAllNodesStatus checks the status of all enabled nodes.
func (a *DashboardController) checkAllNodesStatus(c *gin.Context) {
	err := a.dashboardService.CheckAllNodesStatus()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.dashboard.toasts.checkStatus"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.dashboard.toasts.checkStatusSuccess"), nil)
}

