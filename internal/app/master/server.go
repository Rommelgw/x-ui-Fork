package master

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"x-ui/internal/config"
	"x-ui/internal/model"
	"x-ui/internal/security"
	"x-ui/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	cfg                 *config.MasterConfig
	engine              *gin.Engine
	nodeService         *service.NodeService
	configService       *service.ConfigService
	subscriptionService *service.SubscriptionService
}

func NewServer(cfg *config.MasterConfig, nodeService *service.NodeService, configService *service.ConfigService, subscriptionService *service.SubscriptionService) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &Server{
		cfg:                 cfg,
		engine:              engine,
		nodeService:         nodeService,
		configService:       configService,
		subscriptionService: subscriptionService,
	}

	s.registerRoutes()
	return s
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}

func (s *Server) registerRoutes() {
	api := s.engine.Group("/api")
	{
		api.GET("/health", s.handleHealth)
		api.POST("/nodes/register", s.handleRegisterNode)
		api.GET("/nodes/:node_id/config", s.handleGetNodeConfig)
		api.POST("/nodes/:node_id/stats", s.handleReceiveNodeStats)
		api.GET("/subscriptions/:client_uuid", s.handleSubscriptionBundle)
		admin := api.Group("/admin")
		{
			admin.GET("/dashboard", s.handleDashboard)
			admin.GET("/nodes", s.handleListNodes)
		}
	}
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now()})
}

func (s *Server) handleRegisterNode(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		s.respondError(c, http.StatusBadRequest, "failed to read body")
		return
	}
	defer func() { c.Request.Body = io.NopCloser(bytes.NewBuffer(body)) }()

	signature := c.GetHeader("X-Node-Signature")
	if signature == "" || !security.VerifyHMAC(body, s.cfg.HMACSecret, signature) {
		s.respondError(c, http.StatusUnauthorized, "invalid signature")
		return
	}

	var payload RegisterNodePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		s.respondError(c, http.StatusBadRequest, "invalid payload")
		return
	}

	secret, err := s.ensureNodeSecret(c.Request.Context(), payload.ID)
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	node := &model.Node{
		ID:          payload.ID,
		Name:        payload.Name,
		MasterURL:   payload.MasterURL,
		SecretKey:   secret,
		Status:      model.NodeStatusOnline,
		IPAddress:   payload.IPAddress,
		Hostname:    payload.Hostname,
		Location:    payload.Location,
		XrayVersion: payload.XrayVersion,
		ListenAddr:  payload.ListenAddr,
	}

	if err := s.nodeService.UpsertNode(c.Request.Context(), node); err != nil {
		s.respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"node_id":    node.ID,
			"secret_key": node.SecretKey,
			"status":     node.Status,
		},
	})
}

func (s *Server) handleGetNodeConfig(c *gin.Context) {
	nodeID := c.Param("node_id")
	if !s.verifyNodeSignature(c, nodeID, []byte(c.Request.URL.Path)) {
		s.respondError(c, http.StatusUnauthorized, "invalid signature")
		return
	}

	config, err := s.configService.GetNodeConfiguration(c.Request.Context(), nodeID)
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	response := NodeConfigResponse{
		Inbounds:      config.Inbounds,
		Outbounds:     config.Outbounds,
		Clients:       config.Clients,
		Routing:       config.Routing,
		DNS:           config.DNS,
		Policy:        config.Policy,
		Transport:     config.Transport,
		Log:           config.Log,
		LastUpdatedAt: config.GeneratedAt,
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": response})
}

func (s *Server) handleReceiveNodeStats(c *gin.Context) {
	nodeID := c.Param("node_id")
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		s.respondError(c, http.StatusBadRequest, "failed to read body")
		return
	}
	defer func() { c.Request.Body = io.NopCloser(bytes.NewBuffer(body)) }()

	if !s.verifyNodeSignature(c, nodeID, body) {
		s.respondError(c, http.StatusUnauthorized, "invalid signature")
		return
	}

	var payload NodeStatsPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		s.respondError(c, http.StatusBadRequest, "invalid payload")
		return
	}

	ctx := c.Request.Context()
	status := model.NodeStatus(payload.Status)
	if status == "" {
		status = model.NodeStatusOnline
	}

	if err := s.nodeService.UpdateNodeStatus(ctx, nodeID, status); err != nil {
		s.respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	for _, stat := range payload.ClientStats {
		modelStat := &model.ClientNodeStat{
			ClientID: stat.ClientID,
			Upload:   stat.Upload,
			Download: stat.Download,
			LastUsed: stat.LastUsed,
		}
		if err := s.nodeService.SaveNodeStats(ctx, nodeID, modelStat); err != nil {
			s.respondError(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleSubscriptionBundle(c *gin.Context) {
	clientUUID := c.Param("client_uuid")
	if clientUUID == "" {
		s.respondError(c, http.StatusBadRequest, "client uuid required")
		return
	}

	bundle, err := s.subscriptionService.GenerateBundle(c.Request.Context(), clientUUID)
	if err != nil {
		s.respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	format := strings.ToLower(c.DefaultQuery("format", "json"))
	switch format {
	case "json":
		c.Data(http.StatusOK, "application/json", []byte(bundle.JSON))
	case "clash":
		c.Data(http.StatusOK, "application/yaml", []byte(bundle.Clash))
	case "v2ray", "v2":
		c.Data(http.StatusOK, "text/plain", []byte(bundle.V2Ray))
	case "shadowrocket", "sr":
		c.Data(http.StatusOK, "text/plain", []byte(bundle.Shadowrocket))
	default:
		s.respondError(c, http.StatusBadRequest, "unsupported format")
	}
}

func (s *Server) handleDashboard(c *gin.Context) {
	metrics, err := s.nodeService.GetDashboardMetrics(c.Request.Context())
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": metrics})
}

func (s *Server) handleListNodes(c *gin.Context) {
	nodes, err := s.nodeService.GetAllNodes(c.Request.Context())
	if err != nil {
		s.respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]gin.H, 0, len(nodes))
	for _, node := range nodes {
		groups := make([]gin.H, 0, len(node.Groups))
		for _, group := range node.Groups {
			groups = append(groups, gin.H{
				"id":   group.ID,
				"name": group.Name,
			})
		}

		response = append(response, gin.H{
			"id":           node.ID,
			"name":         node.Name,
			"status":       node.Status,
			"ip_address":   node.IPAddress,
			"hostname":     node.Hostname,
			"location":     node.Location,
			"xray_version": node.XrayVersion,
			"listen_addr":  node.ListenAddr,
			"last_seen":    node.LastSeen,
			"groups":       groups,
		})
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": response})
}

func (s *Server) ensureNodeSecret(ctx context.Context, nodeID string) (string, error) {
	if nodeID == "" {
		return "", errors.New("node id is required")
	}

	existing, err := s.nodeService.GetNode(ctx, nodeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return security.GenerateSecret(32)
		}
		return "", err
	}
	return existing.SecretKey, nil
}

func (s *Server) verifyNodeSignature(c *gin.Context, nodeID string, payload []byte) bool {
	if nodeID == "" {
		return false
	}
	ctx := c.Request.Context()
	node, err := s.nodeService.GetNode(ctx, nodeID)
	if err != nil {
		return false
	}
	signature := c.GetHeader("X-Node-Signature")
	if signature == "" {
		return false
	}
	return security.VerifyHMAC(payload, node.SecretKey, signature)
}

func (s *Server) respondError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"success": false, "message": message})
}
