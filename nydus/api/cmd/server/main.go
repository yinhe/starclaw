package main

import (
	"fmt"
	"log"
	"os"
	"time"

	pheromone "starclaw.net/pheromone/sdk"

	"starclaw.net/nydus/api/internal/config"
	"starclaw.net/nydus/api/internal/database"
	"starclaw.net/nydus/api/internal/handler"
	"starclaw.net/nydus/api/internal/router"
)

func main() {
	cfgPath := "nydus.yaml"
	if v := os.Getenv("NYDUS_CONFIG"); v != "" {
		cfgPath = v
	}
	config.LoadServer(cfgPath)

	// Init SQLite database
	database.Init(config.C.Server.DBPath)

	// Sync YAML repos → DB (idempotent)
	database.SyncYAMLRepos()

	// Ensure repos dir exists
	os.MkdirAll(config.C.Server.ReposDir, 0755)

	// Init bare repos from config
	for name := range config.C.Repos {
		handler.InitBareRepo(name)
	}

	if config.C.Server.Secret == "" {
		log.Fatal("[nydus] server.secret must be set in config")
	}

	// Connect to Pheromone ESB
	natsURL := os.Getenv("PHEROMONE_NATS_URL")
	if natsURL == "" {
		natsURL = "nats://127.0.0.1:4222"
	}
	ph, err := pheromone.New(natsURL, pheromone.ServiceInfo{
		Name:    "nydus",
		Version: "1.0.0",
		Port:    8085,
		Tags:    []string{"git", "deploy", "registry"},
	})
	if err != nil {
		log.Printf("[nydus] pheromone connect failed (non-fatal): %v", err)
	} else {
		ph.StartHeartbeat(30 * time.Second)
		defer ph.Close()
		log.Printf("[nydus] pheromone ESB connected (%s)", natsURL)
	}

	r := router.Setup()

	port := config.C.Server.Port
	log.Printf("[nydus] Nydus Server starting on :%s", port)
	log.Printf("[nydus] SSH clone: git clone %s@<host>:<repo>.git", config.C.Server.SSHUser)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatal(err)
	}
}
