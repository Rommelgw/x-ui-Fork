package config

import (
    "fmt"
    "os"
    "strconv"
)

type MasterConfig struct {
    HTTPPort    string
    DBDriver    string
    DBDSN       string
    AutoMigrate bool
    HMACSecret  string
    TLSCertFile string
    TLSKeyFile  string
}

func LoadMasterConfig() (*MasterConfig, error) {
    cfg := &MasterConfig{
        HTTPPort:    getenv("MASTER_HTTP_PORT", "8085"),
        DBDriver:    getenv("MASTER_DB_DRIVER", "sqlite"),
        DBDSN:       os.Getenv("MASTER_DB_DSN"),
        AutoMigrate: getenvBool("MASTER_DB_AUTO_MIGRATE", true),
        HMACSecret:  os.Getenv("MASTER_HMAC_SECRET"),
        TLSCertFile: os.Getenv("MASTER_TLS_CERT_FILE"),
        TLSKeyFile:  os.Getenv("MASTER_TLS_KEY_FILE"),
    }

    if cfg.DBDriver != "sqlite" && cfg.DBDSN == "" {
        return nil, fmt.Errorf("MASTER_DB_DSN is required when using %s driver", cfg.DBDriver)
    }

    if cfg.DBDriver == "sqlite" && cfg.DBDSN == "" {
        cfg.DBDSN = "data/master.db"
    }

    if cfg.HMACSecret == "" {
        return nil, fmt.Errorf("MASTER_HMAC_SECRET must be provided")
    }

    return cfg, nil
}

func getenv(key string, fallback string) string {
    if value, ok := os.LookupEnv(key); ok {
        return value
    }
    return fallback
}

func getenvBool(key string, fallback bool) bool {
    if value, ok := os.LookupEnv(key); ok {
        parsed, err := strconv.ParseBool(value)
        if err == nil {
            return parsed
        }
    }
    return fallback
}

