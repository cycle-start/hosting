package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL    string
	JWTSecret      string
	JWTIssuer      string
	HTTPListenAddr string
	LogLevel       string
	CORSOrigins    []string
	HostingAPIURL  string
	HostingAPIKey  string
	DevMode        bool
}

func Load() (*Config, error) {
	origins := getEnv("CORS_ORIGINS", "http://localhost:5173")
	var corsList []string
	for _, o := range strings.Split(origins, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			corsList = append(corsList, trimmed)
		}
	}

	cfg := &Config{
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		JWTIssuer:      getEnv("JWT_ISSUER", "controlpanel-api"),
		HTTPListenAddr: getEnv("HTTP_LISTEN_ADDR", ":8080"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		CORSOrigins:    corsList,
		HostingAPIURL:  getEnv("HOSTING_API_URL", ""),
		HostingAPIKey:  getEnv("HOSTING_API_KEY", ""),
		DevMode:        getEnv("DEV_MODE", "") == "true",
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	return nil
}

// HostingWSURL returns the hosting API URL with the scheme changed to ws(s).
func (c *Config) HostingWSURL() string {
	u := c.HostingAPIURL
	if strings.HasPrefix(u, "https://") {
		return "wss://" + u[len("https://"):]
	}
	if strings.HasPrefix(u, "http://") {
		return "ws://" + u[len("http://"):]
	}
	return u
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
