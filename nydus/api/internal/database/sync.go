package database

import (
	"log"

	"starclaw.net/nydus/api/internal/config"
	"starclaw.net/nydus/api/internal/model"
)

// SyncYAMLRepos ensures every repo defined in nydus.yaml exists in the DB.
// Existing DB entries are not overwritten (DB is source of truth for metadata).
func SyncYAMLRepos() {
	for name, rc := range config.C.Repos {
		var existing model.NydusRepo
		if err := DB.Where("name = ?", name).First(&existing).Error; err == nil {
			continue // already in DB
		}
		repo := model.NydusRepo{
			Name:        name,
			Description: rc.Description,
			Public:      rc.Public,
			Source:      "system",
			Status:      "active",
		}
		if err := DB.Create(&repo).Error; err != nil {
			log.Printf("[nydus] failed to sync repo %s to DB: %v", name, err)
		} else {
			log.Printf("[nydus] synced YAML repo to DB: %s (public=%v)", name, rc.Public)
		}
	}
}
