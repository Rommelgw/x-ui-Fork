package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	agentconfig "x-ui/internal/agent/config"
	"x-ui/internal/security"

	"github.com/gorilla/mux"
)

type Agent struct {
	configPath   string
	config       *agentconfig.Config
	sharedSecret atomic.Value
	httpClient   *http.Client
	logger       *log.Logger

	stateOnce      sync.Once
	stateMu        sync.RWMutex
	runtimeConfig  *RuntimeConfig
	xrayConfigDir  string
	xrayConfigPath string
	xrayLogDir     string
}

func New(configPath string, cfg *agentconfig.Config) *Agent {
	client := &http.Client{Timeout: 30 * time.Second}
	logger := log.Default()

	a := &Agent{
		configPath: configPath,
		config:     cfg,
		httpClient: client,
		logger:     logger,
	}

	if cfg.SecretKey != "" {
		a.sharedSecret.Store(cfg.SecretKey)
	}

	return a
}

func (a *Agent) Run(ctx context.Context) error {
	a.logger.Printf("starting node agent (%s)", a.config.String())

	if err := a.register(ctx); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go a.startSchedulers(ctx)

	return a.serve(ctx)
}

func (a *Agent) register(ctx context.Context) error {
	payload := map[string]string{
		"id":           a.config.NodeID,
		"name":         a.config.NodeName,
		"master_url":   a.config.MasterURL,
		"ip_address":   detectPrimaryIP(),
		"hostname":     detectHostname(),
		"location":     "",
		"xray_version": a.config.XrayVersion,
		"listen_addr":  a.config.ListenAddr,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/nodes/register", a.config.MasterURL), bytes.NewReader(body))
	if err != nil {
		return err
	}

	signature := security.ComputeHMAC(body, a.config.RegistrationSecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Signature", signature)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("master responded with status %d", resp.StatusCode)
	}

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			NodeID    string `json:"node_id"`
			SecretKey string `json:"secret_key"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	if !response.Success {
		return errors.New("master responded with failure")
	}

	if response.Data.SecretKey == "" {
		return errors.New("missing secret key in master response")
	}

	a.sharedSecret.Store(response.Data.SecretKey)
	a.config.SecretKey = response.Data.SecretKey

	if err := agentconfig.Save(a.configPath, a.config); err != nil {
		a.logger.Printf("warning: failed to persist secret key to config: %v", err)
	}

	a.logger.Printf("node registered successfully (id=%s)", response.Data.NodeID)
	return nil
}

func (a *Agent) serve(ctx context.Context) error {
	addr := a.config.ListenAddr
	router := mux.NewRouter()
	router.HandleFunc("/api/health", a.wrapHandler(a.healthHandler)).Methods(http.MethodGet)
	router.HandleFunc("/api/sync", a.wrapHandler(a.syncHandler)).Methods(http.MethodPost)
	router.HandleFunc("/api/restart", a.wrapHandler(a.restartHandler)).Methods(http.MethodPost)
	router.HandleFunc("/api/update", a.wrapHandler(a.updateHandler)).Methods(http.MethodPost)

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		<-ctx.Done()
		tCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(tCtx)
	}()

	a.logger.Printf("agent API server listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (a *Agent) wrapHandler(handler func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handler(w, r); err != nil {
			a.logger.Printf("handler error: %v", err)
			status := http.StatusInternalServerError
			var httpErr *HTTPError
			if errors.As(err, &httpErr) {
				status = httpErr.StatusCode
			}
			http.Error(w, err.Error(), status)
		}
	}
}

func (a *Agent) healthHandler(w http.ResponseWriter, r *http.Request) error {
	if err := a.verifyRequestSignature(r, []byte(r.URL.Path)); err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "timestamp": time.Now()})
}

func (a *Agent) syncHandler(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	defer r.Body.Close()

	if err := a.verifyRequestSignature(r, body); err != nil {
		return err
	}

	a.logger.Printf("received sync request (%d bytes)", len(body))

	var req SyncRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return &HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid payload"}
	}

	switch req.Action {
	case "apply_config":
		if err := a.applyXrayConfig(r.Context(), req.Config); err != nil {
			return err
		}
	case "update_clients":
		if err := a.updateClients(r.Context(), req.Config.Clients); err != nil {
			return err
		}
	default:
		return &HTTPError{StatusCode: http.StatusBadRequest, Message: "unknown action"}
	}

	return json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

func (a *Agent) restartHandler(w http.ResponseWriter, r *http.Request) error {
	if err := a.verifyRequestSignature(r, []byte(r.URL.Path)); err != nil {
		return err
	}
	if err := a.restartXray(r.Context()); err != nil {
		return err
	}
	a.logger.Println("received restart request, restarting service")
	return json.NewEncoder(w).Encode(map[string]string{"status": "restarting"})
}

func (a *Agent) updateHandler(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	defer r.Body.Close()

	if err := a.verifyRequestSignature(r, body); err != nil {
		return err
	}
	a.logger.Printf("received update request (%d bytes)", len(body))

	if err := a.ensureXray(r.Context()); err != nil {
		return err
	}
	return json.NewEncoder(w).Encode(map[string]string{"status": "updating"})
}

func (a *Agent) verifyRequestSignature(r *http.Request, payload []byte) error {
	secret, ok := a.sharedSecret.Load().(string)
	if !ok || secret == "" {
		return &HTTPError{StatusCode: http.StatusUnauthorized, Message: "missing shared secret"}
	}
	signature := r.Header.Get("X-Master-Signature")
	if signature == "" {
		return &HTTPError{StatusCode: http.StatusUnauthorized, Message: "missing signature"}
	}
	if !security.VerifyHMAC(payload, secret, signature) {
		return &HTTPError{StatusCode: http.StatusUnauthorized, Message: "invalid signature"}
	}
	return nil
}

func (a *Agent) startSchedulers(ctx context.Context) {
	statsTicker := time.NewTicker(60 * time.Second)
	updateTicker := time.NewTicker(24 * time.Hour)

	for {
		select {
		case <-ctx.Done():
			statsTicker.Stop()
			updateTicker.Stop()
			return
		case <-statsTicker.C:
			a.sendNodeStats(ctx)
		case <-updateTicker.C:
			a.checkForUpdates(ctx)
		}
	}
}

func (a *Agent) sendNodeStats(ctx context.Context) {
	secret, ok := a.sharedSecret.Load().(string)
	if !ok || secret == "" {
		a.logger.Printf("[stats] skipping, secret not negotiated yet")
		return
	}

	stats := a.collectNodeStats(ctx)

	payload := map[string]interface{}{
		"status":       stats.Status,
		"cpu_usage":    stats.CPUUsage,
		"memory_usage": stats.MemoryUsage,
		"online_users": stats.OnlineUsers,
		"clients":      stats.ClientStats,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		a.logger.Printf("[stats] marshal error: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/nodes/%s/stats", a.config.MasterURL, a.config.NodeID), bytes.NewReader(body))
	if err != nil {
		a.logger.Printf("[stats] request error: %v", err)
		return
	}

	signature := security.ComputeHMAC(body, secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Signature", signature)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		a.logger.Printf("[stats] send error: %v", err)
		return
	}
	resp.Body.Close()

	a.logger.Printf("[stats] sent heartbeat (status=%d)", resp.StatusCode)
}

func (a *Agent) checkForUpdates(ctx context.Context) {
	if err := a.ensureXray(ctx); err != nil {
		a.logger.Printf("[update] ensure xray failed: %v", err)
	}
}

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return e.Message
}

func detectHostname() string {
	host, err := os.Hostname()
	if err != nil {
		return ""
	}
	return host
}

func detectPrimaryIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			return ip.String()
		}
	}
	return ""
}
