package database

import (
	"log"

	"starclaw.net/synapse/api/internal/model"
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) {
	if err := db.AutoMigrate(
		&model.User{},
		&model.APIKey{},
		&model.UsageRecord{},
		&model.PaymentOrder{},
		&model.Generation{},
		&model.Role{},
		&model.Permission{},
		&model.UserRole{},
	); err != nil {
		log.Fatalf("[star-ai] auto migrate failed: %v", err)
	}
	log.Println("[star-ai] database migrated")

	// Seed default permissions and roles (idempotent)
	seedRBAC(db)
}

func seedRBAC(db *gorm.DB) {
	// Upsert permissions
	perms := model.DefaultPermissions()
	permMap := map[string]model.Permission{} // name → perm (with ID)
	for _, p := range perms {
		var existing model.Permission
		if err := db.Where("name = ?", p.Name).First(&existing).Error; err != nil {
			db.Create(&p)
			permMap[p.Name] = p
		} else {
			permMap[p.Name] = existing
		}
	}

	// Upsert roles and attach permissions
	roles := model.DefaultRoles()
	for name, def := range roles {
		var role model.Role
		if err := db.Where("name = ?", name).First(&role).Error; err != nil {
			role = model.Role{Name: name, Description: def.Description}
			db.Create(&role)
		}

		// Collect permission objects for this role
		var rolePerms []model.Permission
		for _, pn := range def.Permissions {
			if perm, ok := permMap[pn]; ok {
				rolePerms = append(rolePerms, perm)
			}
		}
		// Replace associations
		if err := db.Model(&role).Association("Permissions").Replace(rolePerms); err != nil {
			log.Printf("[star-ai] warning: failed to set permissions for role %s: %v", name, err)
		}
	}

	log.Println("[star-ai] RBAC seeded")
}
