// Package service provides business logic for managing multi-subscriptions.
package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
	"github.com/mhsanaei/3x-ui/v2/util/random"

	"gorm.io/gorm"
)

// MultiSubscriptionService provides business logic for managing multi-subscriptions.
type MultiSubscriptionService struct {
	nodeService NodeService
}

// AddMultiSubscription adds a new multi-subscription to the database.
func (s *MultiSubscriptionService) AddMultiSubscription(ms *model.MultiSubscription) error {
	db := database.GetDB()

	if ms.Name == "" {
		return common.NewError("multi-subscription name is required")
	}
	if ms.SubId == "" {
		// Generate a unique subId if not provided
		ms.SubId = "multi-" + random.Seq(16)
	}

	// Validate nodeIds JSON
	if ms.NodeIds != "" {
		var nodeIds []int
		if err := json.Unmarshal([]byte(ms.NodeIds), &nodeIds); err != nil {
			return common.NewError("invalid nodeIds JSON format")
		}
		// Verify that all nodes exist
		for _, nodeId := range nodeIds {
			_, err := s.nodeService.GetNode(nodeId)
			if err != nil {
				return common.NewErrorf("node with id %d does not exist", nodeId)
			}
		}
	}

	// Check if subId already exists
	var existing model.MultiSubscription
	err := db.Where("sub_id = ?", ms.SubId).First(&existing).Error
	if err == nil {
		return common.NewError("subscription ID already exists")
	} else if err != gorm.ErrRecordNotFound {
		return err
	}

	// Set defaults
	if ms.CreatedAt == 0 {
		ms.CreatedAt = time.Now().Unix()
	}
	if ms.UpdatedAt == 0 {
		ms.UpdatedAt = time.Now().Unix()
	}

	return db.Create(ms).Error
}

// UpdateMultiSubscription updates an existing multi-subscription.
func (s *MultiSubscriptionService) UpdateMultiSubscription(ms *model.MultiSubscription) error {
	db := database.GetDB()

	if ms.Id <= 0 {
		return common.NewError("multi-subscription ID is required")
	}
	if ms.Name == "" {
		return common.NewError("multi-subscription name is required")
	}

	// Validate nodeIds JSON if provided
	if ms.NodeIds != "" {
		var nodeIds []int
		if err := json.Unmarshal([]byte(ms.NodeIds), &nodeIds); err != nil {
			return common.NewError("invalid nodeIds JSON format")
		}
		// Verify that all nodes exist
		for _, nodeId := range nodeIds {
			_, err := s.nodeService.GetNode(nodeId)
			if err != nil {
				return common.NewErrorf("node with id %d does not exist", nodeId)
			}
		}
	}

	// Check if subId is being changed and if it already exists
	if ms.SubId != "" {
		var existing model.MultiSubscription
		err := db.Where("sub_id = ? AND id != ?", ms.SubId, ms.Id).First(&existing).Error
		if err == nil {
			return common.NewError("subscription ID already exists")
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
	}

	ms.UpdatedAt = time.Now().Unix()

	return db.Save(ms).Error
}

// DeleteMultiSubscription deletes a multi-subscription.
func (s *MultiSubscriptionService) DeleteMultiSubscription(id int) error {
	db := database.GetDB()
	return db.Delete(&model.MultiSubscription{}, id).Error
}

// GetMultiSubscription retrieves a multi-subscription by ID.
func (s *MultiSubscriptionService) GetMultiSubscription(id int) (*model.MultiSubscription, error) {
	db := database.GetDB()
	var ms model.MultiSubscription
	err := db.First(&ms, id).Error
	if err != nil {
		return nil, err
	}
	return &ms, nil
}

// GetMultiSubscriptionBySubId retrieves a multi-subscription by subId.
func (s *MultiSubscriptionService) GetMultiSubscriptionBySubId(subId string) (*model.MultiSubscription, error) {
	db := database.GetDB()
	var ms model.MultiSubscription
	err := db.Where("sub_id = ?", subId).First(&ms).Error
	if err != nil {
		return nil, err
	}
	return &ms, nil
}

// GetAllMultiSubscriptions retrieves all multi-subscriptions.
func (s *MultiSubscriptionService) GetAllMultiSubscriptions() ([]*model.MultiSubscription, error) {
	db := database.GetDB()
	var mss []*model.MultiSubscription
	err := db.Find(&mss).Error
	if err != nil {
		return nil, err
	}
	return mss, nil
}

// GetEnabledMultiSubscriptions retrieves all enabled multi-subscriptions.
func (s *MultiSubscriptionService) GetEnabledMultiSubscriptions() ([]*model.MultiSubscription, error) {
	db := database.GetDB()
	var mss []*model.MultiSubscription
	err := db.Where("enable = ?", true).Find(&mss).Error
	if err != nil {
		return nil, err
	}
	return mss, nil
}

// GetNodeIds retrieves the list of node IDs from a multi-subscription.
func (s *MultiSubscriptionService) GetNodeIds(ms *model.MultiSubscription) ([]int, error) {
	if ms.NodeIds == "" {
		return []int{}, nil
	}
	var nodeIds []int
	err := json.Unmarshal([]byte(ms.NodeIds), &nodeIds)
	if err != nil {
		return nil, common.NewError("invalid nodeIds JSON format")
	}
	return nodeIds, nil
}

// GetNodes retrieves the list of nodes from a multi-subscription.
func (s *MultiSubscriptionService) GetNodes(ms *model.MultiSubscription) ([]*model.Node, error) {
	nodeIds, err := s.GetNodeIds(ms)
	if err != nil {
		return nil, err
	}

	var nodes []*model.Node
	for _, nodeId := range nodeIds {
		node, err := s.nodeService.GetNode(nodeId)
		if err != nil {
			logger.Warningf("Failed to get node %d: %v", nodeId, err)
			continue
		}
		if node.Enable {
			nodes = append(nodes, node)
		}
	}

	return nodes, nil
}

// ValidateMultiSubscription validates a multi-subscription configuration.
func (s *MultiSubscriptionService) ValidateMultiSubscription(ms *model.MultiSubscription) error {
	if ms.Name == "" {
		return common.NewError("multi-subscription name is required")
	}
	if ms.NodeIds == "" {
		return common.NewError("at least one node must be selected")
	}

	nodeIds, err := s.GetNodeIds(ms)
	if err != nil {
		return err
	}

	if len(nodeIds) == 0 {
		return common.NewError("at least one node must be selected")
	}

	// Verify that all nodes exist and are enabled
	for _, nodeId := range nodeIds {
		node, err := s.nodeService.GetNode(nodeId)
		if err != nil {
			return common.NewErrorf("node with id %d does not exist", nodeId)
		}
		if !node.Enable {
			return common.NewErrorf("node with id %d is disabled", nodeId)
		}
	}

	return nil
}

// BuildSubscriptionURL constructs the subscription URL for a given subId based on current settings.
func (s *MultiSubscriptionService) BuildSubscriptionURL(c *gin.Context, subId string) (string, error) {
	if subId == "" {
		return "", common.NewError("subId is required")
	}

	settingService := SettingService{}
	
	// Get subscription settings
	subDomain, _ := settingService.GetSubDomain()
	subPort, _ := settingService.GetSubPort()
	subPath, _ := settingService.GetSubPath()
	subKeyFile, _ := settingService.GetSubKeyFile()
	subCertFile, _ := settingService.GetSubCertFile()
	
	// Determine scheme from TLS configuration
	scheme := "http"
	if subKeyFile != "" && subCertFile != "" {
		scheme = "https"
	}
	
	// Use request scheme and host as fallback if subDomain is not configured
	if subDomain == "" {
		// Try to get from request
		if c != nil {
			scheme = c.GetString("scheme")
			if scheme == "" {
				if c.Request.TLS != nil {
					scheme = "https"
				} else {
					scheme = "http"
				}
			}
			host := c.GetHeader("Host")
			if host == "" {
				host = c.Request.Host
			}
			if host != "" {
				// Use request host:port
				subDomain = host
				subPort = 0 // Don't append port if using request host
			}
		}
		if subDomain == "" {
			// Final fallback
			subDomain = "localhost"
			if subPort == 0 {
				subPort = 2096
			}
		}
	}
	
	// Build host with port
	host := subDomain
	if subPort > 0 && subPort != 80 && subPort != 443 {
		host = fmt.Sprintf("%s:%d", subDomain, subPort)
	} else if (subPort == 443 && scheme == "https") || (subPort == 80 && scheme == "http") {
		// Standard ports, no need to include in URL
	} else if subPort > 0 {
		host = fmt.Sprintf("%s:%d", subDomain, subPort)
	}
	
	// Ensure path format
	if !strings.HasPrefix(subPath, "/") {
		subPath = "/" + subPath
	}
	if !strings.HasSuffix(subPath, "/") {
		subPath = subPath + "/"
	}
	
	// Build final URL
	url := fmt.Sprintf("%s://%s%s%s", scheme, host, subPath, subId)
	return url, nil
}

