package config

import (
	"os"
	"strings"
)

type Config struct {
	Port         string
	DBPath       string
	NydusURL     string
	NydusSecret  string
	OverlordURL  string
	DevBridgeURL string
	GithubToken  string
	LLMBaseURL   string // for PRD generation / Sprint planning
	LLMAPIKey    string
	LLMModel     string
	Secret       string            // JWT signing key
	Whitelist    map[string]string // node_id → password
}

func Load() *Config {
	return &Config{
		Port:         envOr("FORGE_PORT", "8099"),
		DBPath:       envOr("FORGE_DB_PATH", "forge.db"),
		NydusURL:     envOr("NYDUS_URL", "https://nydus.starclaw.net"),
		NydusSecret:  os.Getenv("NYDUS_SECRET"),
		OverlordURL:  envOr("OVERLORD_URL", "http://localhost:8098"),
		DevBridgeURL: envOr("DEVBRIDGE_URL", "http://localhost:9102"),
		GithubToken:  os.Getenv("GITHUB_TOKEN"),
		LLMBaseURL:   envOr("FORGE_LLM_BASE_URL", "https://api.star-ai.net/v1"),
		LLMAPIKey:    os.Getenv("FORGE_LLM_API_KEY"),
		LLMModel:     envOr("FORGE_LLM_MODEL", "deepseek-chat"),
		Secret:       envOr("FORGE_SECRET", "forge-default-secret-change-me"),
		Whitelist:    parseWhitelist(os.Getenv("FORGE_WHITELIST")),
	}
}

// parseWhitelist parses "node1:pass1,node2:pass2" into map.
func parseWhitelist(s string) map[string]string {
	m := make(map[string]string)
	if s == "" {
		return m
	}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if idx := strings.Index(pair, ":"); idx > 0 {
			m[strings.TrimSpace(pair[:idx])] = strings.TrimSpace(pair[idx+1:])
		}
	}
	return m
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
