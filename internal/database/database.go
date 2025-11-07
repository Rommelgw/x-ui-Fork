package database

import (
    "fmt"

    "x-ui/internal/config"

    "gorm.io/driver/mysql"
    "gorm.io/driver/postgres"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

func Connect(cfg *config.MasterConfig) (*gorm.DB, error) {
    var (
        dialector gorm.Dialector
        err       error
    )

    switch cfg.DBDriver {
    case "mysql":
        dialector = mysql.Open(cfg.DBDSN)
    case "postgres":
        dialector = postgres.Open(cfg.DBDSN)
    case "sqlite":
        dialector = sqlite.Open(cfg.DBDSN)
    default:
        return nil, fmt.Errorf("unsupported database driver: %s", cfg.DBDriver)
    }

    db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
    if err != nil {
        return nil, err
    }

    return db, nil
}

