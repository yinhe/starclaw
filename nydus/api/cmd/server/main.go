package main

import (
	"fmt"
	"log"
	"os"

	"github.com/yinhe/starclaw/nydus/internal/config"
	"github.com/yinhe/starclaw/nydus/internal/handler"
	"github.com/yinhe/starclaw/nydus/internal/router"
)

func main() {
	cfgPath := "nydus.yaml"
	if v := os.Getenv("NYDUS_CONFIG"); v != "" {
		cfgPath = v
	}
	config.LoadServer(cfgPath)

	// Ensure repos dir exists
	os.MkdirAll(config.C.Server.ReposDir, 0755)

	// Init bare repos from config
	for name := range config.C.Repos {
		handler.InitBareRepo(name)
	}

	if config.C.Server.Secret == "" {
		log.Fatal("[nydus] server.secret must be set in config")
	}

	r := router.Setup()

	port := config.C.Server.Port
	log.Printf("[nydus] Nydus Server starting on :%s", port)
	log.Printf("[nydus] SSH clone: git clone %s@<host>:<repo>.git", config.C.Server.SSHUser)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatal(err)
	}
}
