package database

import (
	"log"

	"starclaw.net/nydus/api/internal/config"
	"starclaw.net/nydus/api/internal/model"
)

// SyncYAMLRepos ensures every repo defined in nydus.yaml exists in the DB.
// For existing repos, public and description are synced from YAML (YAML is source of truth for visibility).
func SyncYAMLRepos() {
	for name, rc := range config.C.Repos {
		var existing model.NydusRepo
		if err := DB.Where("name = ?", name).First(&existing).Error; err == nil {
			updates := map[string]interface{}{}
			if existing.Public != rc.Public {
				updates["public"] = rc.Public
			}
			if rc.Description != "" && existing.Description != rc.Description {
				updates["description"] = rc.Description
			}
			if len(updates) > 0 {
				DB.Model(&existing).Updates(updates)
				log.Printf("[nydus] synced repo %s: %v", name, updates)
			} else {
				log.Printf("[nydus] repo %s already exists", name)
			}
			continue
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
