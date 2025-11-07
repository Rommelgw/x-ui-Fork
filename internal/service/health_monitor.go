package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"x-ui/internal/model"
	"x-ui/internal/security"
)

type HealthMonitor struct {
	nodeService *NodeService
	httpClient  *http.Client
	interval    time.Duration
}

func NewHealthMonitor(nodeService *NodeService) *HealthMonitor {
	return &HealthMonitor{
		nodeService: nodeService,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		interval:    30 * time.Second,
	}
}

func (h *HealthMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(h.interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.checkAll(ctx)
			}
		}
	}()
}

func (h *HealthMonitor) checkAll(ctx context.Context) {
	nodes, err := h.nodeService.GetAllNodes(ctx)
	if err != nil {
		return
	}

	for _, node := range nodes {
		n := node
		go h.checkNode(ctx, n)
	}
}

func (h *HealthMonitor) checkNode(ctx context.Context, node *model.Node) {
	url, err := buildNodeURL(node)
	if err != nil {
		_ = h.nodeService.UpdateNodeStatus(ctx, node.ID, model.NodeStatusOffline)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}

	signature := security.ComputeHMAC([]byte("/api/health"), node.SecretKey)
	req.Header.Set("X-Master-Signature", signature)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		_ = h.nodeService.UpdateNodeStatus(ctx, node.ID, model.NodeStatusOffline)
		return
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_ = h.nodeService.UpdateNodeStatus(ctx, node.ID, model.NodeStatusOffline)
		return
	}

	status := model.NodeStatusOnline
	if !h.checkXray(ctx, node) {
		status = model.NodeStatusDegraded
	}

	_ = h.nodeService.UpdateNodeStatus(ctx, node.ID, status)
}

func (h *HealthMonitor) checkXray(ctx context.Context, node *model.Node) bool {
	host := firstNonEmpty(node.IPAddress, node.Hostname)
	if host == "" {
		return false
	}

	url := fmt.Sprintf("http://%s:8081/stats", host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func buildNodeURL(node *model.Node) (string, error) {
	host := firstNonEmpty(node.IPAddress, node.Hostname)
	if host == "" {
		return "", errors.New("missing host")
	}

	listen := node.ListenAddr
	if listen == "" {
		listen = ":8080"
	}

	parsedHost, parsedPort, err := net.SplitHostPort(listen)
	if err != nil {
		if strings.HasPrefix(listen, ":") {
			parsedPort = strings.TrimPrefix(listen, ":")
		} else if strings.Contains(listen, ":") {
			// If parsing failed but string contains colon (e.g. IPv6 without brackets), fall back to string operations
			parts := strings.Split(listen, ":")
			parsedHost = strings.Join(parts[:len(parts)-1], ":")
			parsedPort = parts[len(parts)-1]
		} else {
			parsedHost = listen
		}
	}

	if parsedHost == "" {
		parsedHost = host
	}
	if parsedPort == "" {
		parsedPort = "8080"
	}

	return fmt.Sprintf("http://%s:%s/api/health", parsedHost, parsedPort), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
