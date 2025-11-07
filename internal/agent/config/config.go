package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type Config struct {
	MasterURL          string `json:"master_url"`
	NodeID             string `json:"node_id"`
	NodeName           string `json:"node_name"`
	RegistrationSecret string `json:"registration_secret"`
	SecretKey          string `json:"secret_key"`
	XrayVersion        string `json:"xray_version"`
	InstallPath        string `json:"install_path"`
	ListenAddr         string `json:"listen_addr"`
	LogLevel           string `json:"log_level"`
}

func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, err
	}

	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func Save(path string, cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	return nil
}

func (c *Config) ApplyDefaults() {
	if c.ListenAddr == "" {
		c.ListenAddr = ":8080"
	}
	if c.InstallPath == "" {
		c.InstallPath = "/usr/local/bin"
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
}

func (c *Config) Validate() error {
	if c.MasterURL == "" {
		return errors.New("master_url is required")
	}
	if c.NodeID == "" {
		return errors.New("node_id is required")
	}
	if c.NodeName == "" {
		return errors.New("node_name is required")
	}
	if c.RegistrationSecret == "" {
		return errors.New("registration_secret is required")
	}
	return nil
}

func (c *Config) String() string {
	return fmt.Sprintf("node_id=%s master=%s listen=%s", c.NodeID, c.MasterURL, c.ListenAddr)
}

