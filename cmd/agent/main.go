package main

import (
	"context"
	"flag"
	"log"

	"x-ui/internal/agent"
	agentconfig "x-ui/internal/agent/config"
)

func main() {
	configPath := flag.String("config", "/etc/node-agent/config.json", "Path to agent configuration file")
	flag.Parse()

	cfg, err := agentconfig.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	a := agent.New(*configPath, cfg)

	if err := a.Run(context.Background()); err != nil {
		log.Fatalf("agent stopped with error: %v", err)
	}
}

