package main

import (
	"fmt"
	"log"
	"os"

	"starclaw.net/synapse/api/internal/config"
	"starclaw.net/synapse/api/internal/database"
	"starclaw.net/synapse/api/internal/model"
)

// set-admin promotes a user to super_admin by email or phone.
// Usage: go run ./cmd/set-admin <email_or_phone>
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: set-admin <email_or_phone>\n")
		os.Exit(1)
	}
	identifier := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := database.InitMySQL(cfg)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}

	database.AutoMigrate(db)

	// Find user by email or phone
	var user model.User
	if err := db.Where("email = ? OR phone = ?", identifier, identifier).First(&user).Error; err != nil {
		log.Fatalf("user not found: %s", identifier)
	}

	// Set is_admin flag
	db.Model(&user).Update("is_admin", true)

	// Find super_admin role
	var role model.Role
	if err := db.Where("name = ?", "super_admin").First(&role).Error; err != nil {
		log.Fatalf("super_admin role not found — run server first to seed RBAC")
	}

	// Assign role (idempotent)
	var existing model.UserRole
	if err := db.Where("user_id = ? AND role_id = ?", user.ID, role.ID).First(&existing).Error; err != nil {
		ur := model.UserRole{UserID: user.ID, RoleID: role.ID}
		if err := db.Create(&ur).Error; err != nil {
			log.Fatalf("failed to assign role: %v", err)
		}
	}

	fmt.Printf("✅ User '%s' (%s) is now super_admin\n", user.Name, user.Email)
	fmt.Printf("   ID:      %s\n", user.ID)
	fmt.Printf("   Role:    super_admin (%s)\n", role.ID)
	fmt.Printf("   IsAdmin: true\n")
}
