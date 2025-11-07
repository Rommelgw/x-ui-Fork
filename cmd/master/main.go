package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"x-ui/internal/app/master"
	"x-ui/internal/config"
	"x-ui/internal/database"
	"x-ui/internal/service"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadMasterConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}

	if cfg.AutoMigrate {
		if err := database.AutoMigrate(db); err != nil {
			log.Fatalf("database migration error: %v", err)
		}
	}

	nodeService := service.NewNodeService(db)
	configService := service.NewConfigService(db)
	subscriptionService := service.NewSubscriptionService(db, configService)
	monitor := service.NewHealthMonitor(nodeService)
	monitor.Start(ctx)

	server := master.NewServer(cfg, nodeService, configService, subscriptionService)
	addr := ":" + cfg.HTTPPort

	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		log.Fatal(server.Engine().RunTLS(addr, cfg.TLSCertFile, cfg.TLSKeyFile))
		return
	}

	log.Fatal(server.Run(addr))
}
