package service

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"starclaw.net/hive/api/config"
)

// MySQLService manages per-instance databases on the shared MySQL server
type MySQLService struct {
	cfg *config.Config
	db  *sql.DB
}

func NewMySQLService(cfg *config.Config) (*MySQLService, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/",
		cfg.MySQLRootUser, cfg.MySQLRootPass, cfg.MySQLHost, cfg.MySQLPort)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql connect: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	return &MySQLService{cfg: cfg, db: db}, nil
}

// CreateDatabase creates a database and user for a Claw instance
// Returns (dbName, dbUser, dbPassword, error)
func (m *MySQLService) CreateDatabase(slug string) (string, string, string, error) {
	dbName := fmt.Sprintf("claw_%s", slug)
	dbUser := fmt.Sprintf("claw_%s", slug)
	dbPass := randomPassword(24)

	stmts := []string{
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName),
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%%' IDENTIFIED BY '%s'", dbUser, dbPass),
		fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%%'", dbName, dbUser),
		"FLUSH PRIVILEGES",
	}

	for _, stmt := range stmts {
		if _, err := m.db.Exec(stmt); err != nil {
			return "", "", "", fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}

	log.Printf("[hive] created database %s with user %s", dbName, dbUser)
	return dbName, dbUser, dbPass, nil
}

// DropDatabase removes a database and user for a Claw instance
func (m *MySQLService) DropDatabase(slug string) error {
	dbName := fmt.Sprintf("claw_%s", slug)
	dbUser := fmt.Sprintf("claw_%s", slug)

	stmts := []string{
		fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName),
		fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'", dbUser),
		"FLUSH PRIVILEGES",
	}

	for _, stmt := range stmts {
		if _, err := m.db.Exec(stmt); err != nil {
			log.Printf("[hive] warning: drop %s: %v", dbName, err)
		}
	}

	log.Printf("[hive] dropped database %s", dbName)
	return nil
}

// DatabaseSize returns the size of a database in bytes
func (m *MySQLService) DatabaseSize(dbName string) (int64, error) {
	var size sql.NullFloat64
	err := m.db.QueryRow(`
		SELECT SUM(data_length + index_length) 
		FROM information_schema.tables 
		WHERE table_schema = ?`, dbName).Scan(&size)
	if err != nil {
		return 0, err
	}
	if size.Valid {
		return int64(size.Float64), nil
	}
	return 0, nil
}

func randomPassword(length int) string {
	b := make([]byte, length/2+1)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}
