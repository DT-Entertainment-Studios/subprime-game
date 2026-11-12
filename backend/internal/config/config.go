// Package config loads the server configuration from environment variables
// (Spec.md §6). All problems are collected and reported together.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AuthHeaders are the identity header names injected by the OAuth2 proxy.
type AuthHeaders struct {
	User   string
	Name   string
	Email  string
	Groups string
}

// Config is the complete server configuration.
type Config struct {
	Port      int
	StaticDir string
	Auth      AuthHeaders
	LogLevel  string
}

// Load reads the configuration using getenv. It trims every value; empty or
// whitespace-only values fall back to their defaults.
func Load(getenv func(string) string) (Config, error) {
	value := func(key, fallback string) string {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			return fallback
		}
		return v
	}
	cfg := Config{
		StaticDir: value("STATIC_DIR", ""),
		LogLevel:  strings.ToLower(value("LOG_LEVEL", "info")),
		Auth: AuthHeaders{
			User:   value("AUTH_HEADER_USER", "X-Forwarded-User"),
			Name:   value("AUTH_HEADER_NAME", "X-Forwarded-Preferred-Username"),
			Email:  value("AUTH_HEADER_EMAIL", "X-Forwarded-Email"),
			Groups: value("AUTH_HEADER_GROUPS", "X-Forwarded-Groups"),
		},
	}
	var problems []string
	portText := value("PORT", "8080")
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		problems = append(problems, fmt.Sprintf("PORT %q is not a valid port", portText))
	} else {
		cfg.Port = port
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		problems = append(problems, fmt.Sprintf("LOG_LEVEL %q is not one of debug, info, warn, error", cfg.LogLevel))
	}
	for key, header := range map[string]string{
		"AUTH_HEADER_USER":   cfg.Auth.User,
		"AUTH_HEADER_NAME":   cfg.Auth.Name,
		"AUTH_HEADER_EMAIL":  cfg.Auth.Email,
		"AUTH_HEADER_GROUPS": cfg.Auth.Groups,
	} {
		if strings.ContainsAny(header, " \t\r\n") {
			problems = append(problems, fmt.Sprintf("%s %q contains whitespace", key, header))
		}
	}
	if cfg.StaticDir != "" {
		info, err := os.Stat(cfg.StaticDir)
		if err != nil {
			problems = append(problems, fmt.Sprintf("STATIC_DIR %q does not exist", cfg.StaticDir))
		} else if !info.IsDir() {
			problems = append(problems, fmt.Sprintf("STATIC_DIR %q is not a directory", cfg.StaticDir))
		}
	}
	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %s", strings.Join(problems, "; "))
	}
	return cfg, nil
}

// LoadEnv loads the configuration from the process environment.
func LoadEnv() (Config, error) { return Load(os.Getenv) }
