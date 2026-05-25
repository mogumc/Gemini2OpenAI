package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the proxy server.
type Config struct {
	// Server
	Port string

	// Gemini
	GeminiAPIKey  string
	GeminiBaseURL string
	GeminiModel   string

	// Proxy auth
	ProxyAPIKey string

	// Defaults for OpenAI-incompatible features
	DefaultTemperature     float64
	DefaultMaxTokens       int
	DefaultTopP            float64
	DefaultTopK            int
	DefaultStopSequences   []string
	DefaultSafetySettings  string // block_NONE, block_LOW, block_MEDIUM, block_HIGH
	DefaultThinkingBudget  int    // -1=dynamic, 0=disabled, >0=token limit
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:               getEnv("PORT", "8080"),
		GeminiAPIKey:       os.Getenv("GEMINI_API_KEY"),
		GeminiBaseURL:      getEnv("GEMINI_BASE_URL", "https://generativelanguage.googleapis.com/v1beta"),
		GeminiModel:        getEnv("GEMINI_MODEL", "gemini-2.5-flash"),
		ProxyAPIKey:        os.Getenv("PROXY_API_KEY"),
		DefaultTemperature: getEnvFloat("DEFAULT_TEMPERATURE", 1.0),
		DefaultMaxTokens:   getEnvInt("DEFAULT_MAX_TOKENS", 8192),
		DefaultTopP:        getEnvFloat("DEFAULT_TOP_P", 0.95),
		DefaultTopK:        getEnvInt("DEFAULT_TOP_K", 40),
		DefaultSafetySettings: getEnv("DEFAULT_SAFETY_SETTINGS", "block_NONE"),
		DefaultThinkingBudget: getEnvInt("DEFAULT_THINKING_BUDGET", 0),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		var f float64
		_, err := fmt.Sscanf(v, "%f", &f)
		if err == nil {
			return f
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		_, err := fmt.Sscanf(v, "%d", &i)
		if err == nil {
			return i
		}
	}
	return fallback
}
