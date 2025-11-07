package agent

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

const (
	defaultXrayConfigDir  = "/usr/local/etc/xray"
	defaultXrayConfigPath = defaultXrayConfigDir + "/config.json"
	defaultXrayLogDir     = "/var/log/xray"
	defaultXrayService    = "xray"
)

var errUnsupportedPlatform = errors.New("unsupported platform for automatic Xray installation")

func (a *Agent) initRuntimeState() {
	a.stateOnce.Do(func() {
		if a.runtimeConfig == nil {
			a.runtimeConfig = &RuntimeConfig{}
		}
		if a.xrayConfigDir == "" {
			a.xrayConfigDir = defaultXrayConfigDir
		}
		if a.xrayConfigPath == "" {
			a.xrayConfigPath = defaultXrayConfigPath
		}
		if a.xrayLogDir == "" {
			a.xrayLogDir = defaultXrayLogDir
		}
	})
}

func (a *Agent) ensureXray(ctx context.Context) error {
	a.initRuntimeState()

	if runtime.GOOS != "linux" {
		return errUnsupportedPlatform
	}

	binPath := a.xrayBinaryPath()
	if exists(binPath) {
		if a.config.XrayVersion == "" || strings.EqualFold(a.config.XrayVersion, "latest") {
			return nil
		}

		ok, err := a.checkInstalledVersion(ctx, binPath, a.config.XrayVersion)
		if err != nil {
			a.logger.Printf("[install] failed to check current version: %v", err)
		}
		if ok {
			return nil
		}
		a.logger.Printf("[install] version mismatch, updating to %s", a.config.XrayVersion)
	}

	if err := a.installXray(ctx); err != nil {
		return fmt.Errorf("install xray: %w", err)
	}

	return nil
}

func (a *Agent) installXray(ctx context.Context) error {
	arch, err := resolveArch()
	if err != nil {
		return err
	}

	versionSegment := "latest/download"
	versionTag := a.config.XrayVersion
	if versionTag != "" && !strings.EqualFold(versionTag, "latest") {
		if !strings.HasPrefix(versionTag, "v") {
			versionTag = "v" + versionTag
		}
		versionSegment = fmt.Sprintf("download/%s", versionTag)
	}

	downloadURL := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/%s/Xray-linux-%s.zip", versionSegment, arch)
	a.logger.Printf("[install] downloading Xray from %s", downloadURL)

	tempFile, err := os.CreateTemp("", "xray-*.zip")
	if err != nil {
		return err
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name())
	}()

	if err := downloadTo(ctx, downloadURL, tempFile); err != nil {
		return err
	}

	if err := a.extractXrayArchive(tempFile.Name()); err != nil {
		return err
	}

	if err := a.ensureDirectories(); err != nil {
		return err
	}

	if err := a.ensureSystemdService(ctx); err != nil {
		return err
	}

	return nil
}

func (a *Agent) extractXrayArchive(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	installPath := a.config.InstallPath
	if installPath == "" {
		installPath = "/usr/local/bin"
	}

	if err := os.MkdirAll(installPath, 0o755); err != nil {
		return err
	}

	shareDir := filepath.Join("/usr/local/share", "xray")
	if err := os.MkdirAll(shareDir, 0o755); err != nil {
		return err
	}

	for _, f := range reader.File {
		name := f.Name
		dstPath := ""
		switch {
		case strings.EqualFold(name, "xray"):
			dstPath = filepath.Join(installPath, "xray")
		case strings.EqualFold(name, "geoip.dat"):
			dstPath = filepath.Join(shareDir, "geoip.dat")
		case strings.EqualFold(name, "geosite.dat"):
			dstPath = filepath.Join(shareDir, "geosite.dat")
		default:
			continue
		}

		if err := extractFile(f, dstPath); err != nil {
			return err
		}

		if strings.HasSuffix(dstPath, "/xray") {
			if err := os.Chmod(dstPath, 0o755); err != nil {
				return err
			}
		}
	}

	return nil
}

func extractFile(zf *zip.File, dest string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dest), "tmp-xray-*")
	if err != nil {
		return err
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	if _, err := io.Copy(tmp, rc); err != nil {
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmp.Name(), dest); err != nil {
		return err
	}

	return nil
}

func (a *Agent) ensureDirectories() error {
	if err := os.MkdirAll(a.xrayConfigDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(a.xrayLogDir, 0o755); err != nil {
		return err
	}

	// Ensure config exists
	if !exists(a.xrayConfigPath) {
		defaultCfg := map[string]any{
			"log": map[string]any{
				"access":   filepath.Join(a.xrayLogDir, "access.log"),
				"error":    filepath.Join(a.xrayLogDir, "error.log"),
				"loglevel": "warning",
			},
			"inbounds":  []any{},
			"outbounds": []any{},
		}
		data, _ := json.MarshalIndent(defaultCfg, "", "  ")
		if err := os.WriteFile(a.xrayConfigPath, data, 0o600); err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) ensureSystemdService(ctx context.Context) error {
	servicePath := "/etc/systemd/system/" + defaultXrayService + ".service"
	unit := `[Unit]
Description=Xray Service
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=%s run -config %s
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
`

	content := fmt.Sprintf(unit, a.xrayBinaryPath(), a.xrayConfigPath)
	if err := os.WriteFile(servicePath, []byte(content), 0o644); err != nil {
		return err
	}

	cmds := [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "--now", defaultXrayService},
	}

	for _, args := range cmds {
		if err := runCommand(ctx, args[0], args[1:]...); err != nil {
			return err
		}
	}

	return nil
}

func (a *Agent) restartXray(ctx context.Context) error {
	if err := runCommand(ctx, "systemctl", "restart", defaultXrayService); err != nil {
		return err
	}
	return nil
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w - %s", name, args, err, stderr.String())
	}
	return nil
}

func (a *Agent) checkInstalledVersion(ctx context.Context, binaryPath, desired string) (bool, error) {
	if desired == "" {
		return true, nil
	}

	cmd := exec.CommandContext(ctx, binaryPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	desired = strings.TrimPrefix(desired, "v")
	return strings.Contains(string(output), desired), nil
}

func resolveArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "64", nil
	case "arm64":
		return "arm64-v8a", nil
	case "arm":
		return "arm32-v7a", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}

func downloadTo(ctx context.Context, url string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d while downloading %s", resp.StatusCode, url)
	}

	if _, err := io.Copy(w, resp.Body); err != nil {
		return err
	}

	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (a *Agent) xrayBinaryPath() string {
	installPath := a.config.InstallPath
	if installPath == "" {
		installPath = "/usr/local/bin"
	}
	return filepath.Join(installPath, "xray")
}

func (a *Agent) applyXrayConfig(ctx context.Context, cfg XrayConfig) error {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	runtimeCfg := &RuntimeConfig{
		Inbounds:  cloneGenericSlice(cfg.Inbounds),
		Outbounds: cloneGenericSlice(cfg.Outbounds),
		Clients:   cloneClients(cfg.Clients),
		Routing:   cloneMap(cfg.Routing),
		DNS:       cloneMap(cfg.DNS),
		Policy:    cloneMap(cfg.Policy),
		Transport: cloneMap(cfg.Transport),
		Log:       cloneMap(cfg.Log),
		Extras:    cloneMap(cfg.OtherSections),
	}

	a.runtimeConfig = runtimeCfg
	if err := a.persistXrayConfigLocked(); err != nil {
		return err
	}

	return a.restartXray(ctx)
}

func (a *Agent) updateClients(ctx context.Context, clients []ClientConfig) error {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()

	if a.runtimeConfig == nil {
		return errors.New("configuration not applied yet")
	}

	a.runtimeConfig.Clients = cloneClients(clients)
	if err := a.persistXrayConfigLocked(); err != nil {
		return err
	}

	return a.restartXray(ctx)
}

func (a *Agent) persistXrayConfigLocked() error {
	if a.runtimeConfig == nil {
		return errors.New("runtime configuration is empty")
	}

	cfg := map[string]any{}

	logSection := a.runtimeConfig.Log
	if logSection == nil {
		logSection = map[string]any{}
	}
	if _, ok := logSection["access"]; !ok {
		logSection["access"] = filepath.Join(a.xrayLogDir, "access.log")
	}
	if _, ok := logSection["error"]; !ok {
		logSection["error"] = filepath.Join(a.xrayLogDir, "error.log")
	}
	if _, ok := logSection["loglevel"]; !ok {
		logSection["loglevel"] = strings.ToLower(a.config.LogLevel)
	}
	cfg["log"] = logSection

	inbounds := cloneGenericSlice(a.runtimeConfig.Inbounds)
	injectClientsIntoInbounds(inbounds, a.runtimeConfig.Clients)
	cfg["inbounds"] = inbounds
	cfg["outbounds"] = cloneGenericSlice(a.runtimeConfig.Outbounds)

	if a.runtimeConfig.Routing != nil {
		cfg["routing"] = a.runtimeConfig.Routing
	}
	if a.runtimeConfig.DNS != nil {
		cfg["dns"] = a.runtimeConfig.DNS
	}
	if a.runtimeConfig.Policy != nil {
		cfg["policy"] = a.runtimeConfig.Policy
	}
	if a.runtimeConfig.Transport != nil {
		cfg["transport"] = a.runtimeConfig.Transport
	}
	if a.runtimeConfig.Extras != nil {
		for k, v := range a.runtimeConfig.Extras {
			if _, exists := cfg[k]; exists {
				continue
			}
			cfg[k] = v
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(a.xrayConfigPath, data, 0o600); err != nil {
		return err
	}

	return nil
}

func injectClientsIntoInbounds(inbounds []GenericMap, clients []ClientConfig) {
	if len(clients) == 0 {
		return
	}

	for _, inbound := range inbounds {
		settings, ok := inbound["settings"].(map[string]any)
		if !ok {
			continue
		}

		clientsCopy := cloneClients(clients)
		if _, ok := settings["clients"]; ok {
			settings["clients"] = clientsCopy
			continue
		}
		if _, ok := settings["users"]; ok {
			settings["users"] = clientsCopy
			continue
		}

		protocol, _ := inbound["protocol"].(string)
		switch protocol {
		case "vmess", "vless", "trojan", "shadowsocks":
			settings["clients"] = clientsCopy
		}
	}
}

func cloneGenericSlice(src []GenericMap) []GenericMap {
	if len(src) == 0 {
		return nil
	}
	out := make([]GenericMap, len(src))
	for i, item := range src {
		out[i] = cloneMap(GenericMap(item))
	}
	return out
}

func cloneMap(src GenericMap) GenericMap {
	if src == nil {
		return nil
	}
	out := make(GenericMap, len(src))
	for k, v := range src {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneClients(src []ClientConfig) []ClientConfig {
	if len(src) == 0 {
		return nil
	}
	out := make([]ClientConfig, len(src))
	for i, item := range src {
		out[i] = ClientConfig(cloneMap(GenericMap(item)))
	}
	return out
}

func cloneValue(v any) any {
	switch value := v.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		out := make([]any, len(value))
		for i, elem := range value {
			out[i] = cloneValue(elem)
		}
		return out
	default:
		return value
	}
}

func (a *Agent) collectNodeStats(ctx context.Context) NodeStats {
	cpuUsage, memUsage := sampleSystemUsage(ctx)

	a.stateMu.RLock()
	defer a.stateMu.RUnlock()

	onlineUsers := 0
	if a.runtimeConfig != nil {
		onlineUsers = len(a.runtimeConfig.Clients)
	}

	return NodeStats{
		Status:      "online",
		CPUUsage:    cpuUsage,
		MemoryUsage: memUsage,
		OnlineUsers: onlineUsers,
		ClientStats: nil,
	}
}

func sampleSystemUsage(ctx context.Context) (float64, float64) {
	type result struct {
		cpu float64
		mem float64
	}

	var r result
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		cpuPerc, err := cpuPercentWithContext(ctx)
		if err == nil {
			r.cpu = cpuPerc
		}
	}()

	go func() {
		defer wg.Done()
		memPerc, err := memoryPercentWithContext(ctx)
		if err == nil {
			r.mem = memPerc
		}
	}()

	wg.Wait()
	return r.cpu, r.mem
}

func cpuPercentWithContext(ctx context.Context) (float64, error) {
	values, err := cpu.PercentWithContext(ctx, 200*time.Millisecond, false)
	if err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, nil
	}
	return values[0], nil
}

func memoryPercentWithContext(ctx context.Context) (float64, error) {
	info, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return 0, err
	}
	return info.UsedPercent, nil
}
