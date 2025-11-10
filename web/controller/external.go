package controller

import (
	"net/http"
	"time"

	"github.com/mhsanaei/3x-ui/v2/web/middleware"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

// ExternalController exposes minimal read-only APIs for node-to-node communication secured by X-API-Key.
type ExternalController struct {
	serverService  service.ServerService
	inboundService service.InboundService
}

func NewExternalController(g *gin.RouterGroup) *ExternalController {
	a := &ExternalController{}
	a.initRouter(g)
	return a
}

func (a *ExternalController) initRouter(g *gin.RouterGroup) {
	api := g.Group("/api/external")
	api.Use(middleware.ExternalAPIKeyMiddleware())

	api.GET("/server/status", a.getStatus)
	api.GET("/inbounds/list", a.listInbounds)
	api.GET("/health", a.healthcheck)
}

func (a *ExternalController) getStatus(c *gin.Context) {
	status := a.serverService.GetStatus(nil)
	jsonObj(c, status, nil)
}

func (a *ExternalController) listInbounds(c *gin.Context) {
	inbounds, err := a.inboundService.GetAllInbounds()
	jsonObj(c, inbounds, err)
}

// healthcheck provides a simple health check endpoint for orchestrators (Kubernetes, Docker, etc.)
func (a *ExternalController) healthcheck(c *gin.Context) {
	// Simple health check - just return 200 OK if API key is valid
	// The middleware already validates the API key, so if we reach here, we're healthy
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
	})
}
