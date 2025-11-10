// Package controller provides HTTP request handlers for multi-subscription management.
package controller

import (
	"strconv"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/web/service"

	"github.com/gin-gonic/gin"
)

// MultiSubscriptionController handles HTTP requests related to multi-subscription management.
type MultiSubscriptionController struct {
	BaseController
	multiSubscriptionService service.MultiSubscriptionService
}

// NewMultiSubscriptionController creates a new MultiSubscriptionController and sets up its routes.
func NewMultiSubscriptionController(g *gin.RouterGroup) *MultiSubscriptionController {
	a := &MultiSubscriptionController{
		multiSubscriptionService: service.MultiSubscriptionService{},
	}
	a.initRouter(g)
	return a
}

// initRouter initializes the routes for multi-subscription-related operations.
func (a *MultiSubscriptionController) initRouter(g *gin.RouterGroup) {
	g.GET("/", a.getMultiSubscriptions)
	g.GET("/:id", a.getMultiSubscription)
	g.GET("/subId/:subId", a.getMultiSubscriptionBySubId)
	g.GET("/:id/subscription-url", a.getSubscriptionURL)

	g.POST("/", a.addMultiSubscription)
	g.POST("/:id", a.updateMultiSubscription)
	g.POST("/:id/delete", a.deleteMultiSubscription)
	g.POST("/:id/validate", a.validateMultiSubscription)
}

// getMultiSubscriptions retrieves all multi-subscriptions.
func (a *MultiSubscriptionController) getMultiSubscriptions(c *gin.Context) {
	mss, err := a.multiSubscriptionService.GetAllMultiSubscriptions()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.getMultiSubscriptions"), err)
		return
	}
	jsonObj(c, mss, nil)
}

// getMultiSubscription retrieves a specific multi-subscription by ID.
func (a *MultiSubscriptionController) getMultiSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	ms, err := a.multiSubscriptionService.GetMultiSubscription(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.getMultiSubscription"), err)
		return
	}
	jsonObj(c, ms, nil)
}

// getMultiSubscriptionBySubId retrieves a multi-subscription by subId.
func (a *MultiSubscriptionController) getMultiSubscriptionBySubId(c *gin.Context) {
	subId := c.Param("subId")
	ms, err := a.multiSubscriptionService.GetMultiSubscriptionBySubId(subId)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.getMultiSubscription"), err)
		return
	}
	jsonObj(c, ms, nil)
}

// addMultiSubscription adds a new multi-subscription.
func (a *MultiSubscriptionController) addMultiSubscription(c *gin.Context) {
	var ms model.MultiSubscription
	if err := c.ShouldBindJSON(&ms); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.addMultiSubscription"), err)
		return
	}
	err := a.multiSubscriptionService.AddMultiSubscription(&ms)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.addMultiSubscription"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.addMultiSubscriptionSuccess"), nil)
}

// updateMultiSubscription updates an existing multi-subscription.
func (a *MultiSubscriptionController) updateMultiSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	var ms model.MultiSubscription
	if err := c.ShouldBindJSON(&ms); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.updateMultiSubscription"), err)
		return
	}
	ms.Id = id
	err = a.multiSubscriptionService.UpdateMultiSubscription(&ms)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.updateMultiSubscription"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.updateMultiSubscriptionSuccess"), nil)
}

// deleteMultiSubscription deletes a multi-subscription.
func (a *MultiSubscriptionController) deleteMultiSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	err = a.multiSubscriptionService.DeleteMultiSubscription(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.deleteMultiSubscription"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.deleteMultiSubscriptionSuccess"), nil)
}

// validateMultiSubscription validates a multi-subscription configuration.
func (a *MultiSubscriptionController) validateMultiSubscription(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	ms, err := a.multiSubscriptionService.GetMultiSubscription(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.getMultiSubscription"), err)
		return
	}
	err = a.multiSubscriptionService.ValidateMultiSubscription(ms)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.validateMultiSubscription"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.validateMultiSubscriptionSuccess"), nil)
}

// getSubscriptionURL returns the subscription URL for a multi-subscription.
func (a *MultiSubscriptionController) getSubscriptionURL(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "get"), err)
		return
	}
	ms, err := a.multiSubscriptionService.GetMultiSubscription(id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.getMultiSubscription"), err)
		return
	}
	
	url, err := a.multiSubscriptionService.BuildSubscriptionURL(c, ms.SubId)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.multiSubscriptions.toasts.getSubscriptionURL"), err)
		return
	}
	
	jsonObj(c, map[string]string{"url": url}, nil)
}

