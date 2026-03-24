package config

import "os"

type Config struct {
	Port       string
	DBPath     string
	NydusURL   string
	NydusSecret string
	OverlordURL string
	DevBridgeURL string
	GithubToken string
	LLMBaseURL  string // for PRD generation / Sprint planning
	LLMAPIKey   string
	LLMModel    string
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
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
