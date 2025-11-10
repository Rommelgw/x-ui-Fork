// Package service provides HTTP client for communicating with remote 3x-ui nodes.
package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/util/common"
)

// NodeClient provides HTTP client functionality for communicating with remote 3x-ui nodes.
type NodeClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewNodeClient creates a new NodeClient instance for communicating with a remote node.
func NewNodeClient(node *model.Node) *NodeClient {
	protocol := node.Protocol
	if protocol == "" {
		protocol = "https"
	}
	baseURL := fmt.Sprintf("%s://%s:%d", protocol, node.Host, node.Port)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	return &NodeClient{
		baseURL: baseURL,
		apiKey:  node.ApiKey,
		client:  client,
	}
}

// makeRequest performs an HTTP request to the node external API with authentication.
func (nc *NodeClient) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", nc.baseURL, endpoint)
	startTime := time.Now()

	var reqBody io.Reader
	var bodySize int
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			logger.Errorf("NodeClient [%s] failed to marshal request body: %v", url, err)
			return nil, common.NewError("failed to marshal request body:", err)
		}
		bodySize = len(jsonData)
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		logger.Errorf("NodeClient [%s] failed to create request: %v", url, err)
		return nil, common.NewError("failed to create request:", err)
	}

	// Add API key to header for external API
	if nc.apiKey != "" {
		req.Header.Set("X-API-Key", nc.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	logger.Debugf("NodeClient [%s %s] sending request (body size: %d bytes)", method, url, bodySize)

	resp, err := nc.client.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		logger.Errorf("NodeClient [%s %s] request failed after %v: %v", method, url, duration, err)
		return nil, common.NewError("failed to execute request:", err)
	}

	// Log response details
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Create a new reader from the body bytes for the caller
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	bodyStr := string(bodyBytes)
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500] + "... (truncated)"
	}

	if resp.StatusCode != http.StatusOK {
		logger.Warningf("NodeClient [%s %s] returned status %d after %v. Response body: %s", method, url, resp.StatusCode, duration, bodyStr)
	} else {
		logger.Debugf("NodeClient [%s %s] returned status %d after %v (response size: %d bytes)", method, url, resp.StatusCode, duration, len(bodyBytes))
	}

	return resp, nil
}

// GetStatus retrieves the status of the remote node.
func (nc *NodeClient) GetStatus() (*Status, error) {
	resp, err := nc.makeRequest("GET", "/api/external/server/status", nil)
	if err != nil {
		logger.Errorf("NodeClient GetStatus failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
		logger.Errorf("NodeClient GetStatus: %s", errMsg)
		return nil, common.NewError("failed to get status: " + errMsg)
	}

	var result struct {
		Success bool   `json:"success"`
		Obj     Status `json:"obj"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Errorf("NodeClient GetStatus failed to decode response: %v", err)
		return nil, common.NewError("failed to decode response:", err)
	}

	if !result.Success {
		logger.Warningf("NodeClient GetStatus: node returned error in response")
		return nil, common.NewError("node returned error")
	}

	return &result.Obj, nil
}

// GetInbounds retrieves all inbounds from the remote node.
func (nc *NodeClient) GetInbounds() ([]*model.Inbound, error) {
	resp, err := nc.makeRequest("GET", "/api/external/inbounds/list", nil)
	if err != nil {
		logger.Errorf("NodeClient GetInbounds failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
		logger.Errorf("NodeClient GetInbounds: %s", errMsg)
		return nil, common.NewError("failed to get inbounds: " + errMsg)
	}

	var result struct {
		Success bool             `json:"success"`
		Obj     []*model.Inbound `json:"obj"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logger.Errorf("NodeClient GetInbounds failed to decode response: %v", err)
		return nil, common.NewError("failed to decode response:", err)
	}

	if !result.Success {
		logger.Warningf("NodeClient GetInbounds: node returned error in response")
		return nil, common.NewError("node returned error")
	}

	logger.Debugf("NodeClient GetInbounds: successfully retrieved %d inbounds", len(result.Obj))
	return result.Obj, nil
}

// GetClientsBySubId retrieves inbounds that have a specific subscription ID from the remote node.
func (nc *NodeClient) GetClientsBySubId(subId string) ([]*model.Inbound, error) {
	inbounds, err := nc.GetInbounds()
	if err != nil {
		return nil, err
	}

	var filteredInbounds []*model.Inbound
	for _, inbound := range inbounds {
		var settings map[string]interface{}
		if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
			continue
		}
		clients, ok := settings["clients"].([]interface{})
		if !ok {
			continue
		}
		for _, client := range clients {
			if clientMap, ok := client.(map[string]interface{}); ok {
				if clientSubId, ok := clientMap["subId"].(string); ok && clientSubId == subId {
					filteredInbounds = append(filteredInbounds, inbound)
					break
				}
			}
		}
	}

	return filteredInbounds, nil
}

// CheckConnection checks if the node is accessible and returns its status.
func (nc *NodeClient) CheckConnection() (string, error) {
	status, err := nc.GetStatus()
	if err != nil {
		logger.Warningf("NodeClient CheckConnection failed for %s: %v", nc.baseURL, err)
		return string(model.NodeStatusOffline), err
	}

	nodeStatus := string(model.NodeStatusOffline)
	if status.Xray.State == "running" {
		nodeStatus = string(model.NodeStatusOnline)
		logger.Debugf("NodeClient CheckConnection: node %s is online (xray running)", nc.baseURL)
	} else if status.Xray.State == "error" {
		nodeStatus = string(model.NodeStatusError)
		logger.Warningf("NodeClient CheckConnection: node %s has xray error state", nc.baseURL)
	} else {
		logger.Debugf("NodeClient CheckConnection: node %s is offline (xray state: %s)", nc.baseURL, status.Xray.State)
	}

	return nodeStatus, nil
}

// Ping performs a simple ping to check if the node is reachable.
func (nc *NodeClient) Ping() error {
	_, err := nc.makeRequest("GET", "/api/external/server/status", nil)
	return err
}
