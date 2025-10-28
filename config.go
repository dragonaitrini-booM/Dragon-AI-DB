package main

import (
    "bufio"
    "log"
    "os"
    "strings"
)

// Config holds runtime configuration loaded from the environment or a .env file.
type Config struct {
	SupabaseURL     string // Supabase project URL.
	SupabaseKey     string // Supabase API key.
	CloudflareToken string // Cloudflare API token.
	AIAPIKey        string // API key for an AI service.
	MasterKey       string // Master key for local encryption.
	Port            string // Port for the server to listen on.
	LogLevel        string // Logging level (e.g., "info", "debug").
}

// AppConfig is the global configuration instance, populated by LoadConfig.
var AppConfig Config

// LoadConfig loads configuration from environment variables, falling back to a
// `.env` file in the repository root if it exists. Environment variables always
// take precedence over values in the `.env` file. It sets sensible defaults for
// Port and LogLevel if they are not specified.
//
// It validates the presence of critical secrets and logs a warning if they are
// missing, but does not prevent the application from starting.
func LoadConfig() {
	// Try to load .env if present (simple parser)
	if _, err := os.Stat(".env"); err == nil {
		f, err := os.Open(".env")
		if err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if idx := strings.Index(line, "="); idx > 0 {
					k := strings.TrimSpace(line[:idx])
					v := strings.TrimSpace(line[idx+1:])
					// remove optional surrounding quotes
					v = strings.Trim(v, `"'`)
					// only set if not present in env (env has precedence)
					if _, ok := os.LookupEnv(k); !ok {
						os.Setenv(k, v)
					}
				}
			}
			f.Close()
		}
	}

	AppConfig = Config{
		SupabaseURL:     os.Getenv("SUPABASE_URL"),
		SupabaseKey:     os.Getenv("SUPABASE_KEY"),
		CloudflareToken: os.Getenv("CLOUDFLARE_API_TOKEN"),
		AIAPIKey:        os.Getenv("AI_API_KEY"),
		MasterKey:       os.Getenv("MASTER_KEY"),
		Port:            os.Getenv("PORT"),
		LogLevel:        os.Getenv("LOG_LEVEL"),
	}

	// sensible defaults
	if AppConfig.Port == "" {
		AppConfig.Port = "8080"
	}
	if AppConfig.LogLevel == "" {
		AppConfig.LogLevel = "info"
	}

	// Validate critical secrets and warn if missing
	if AppConfig.MasterKey == "" {
		log.Println("WARNING: MASTER_KEY is not set. Local encryption will be insecure.")
	}

	// Don't log raw API keys — redact when displaying summary
	redactedAI := "(not configured)"
	if AppConfig.AIAPIKey != "" {
		redactedAI = "(configured)"
	}

	log.Printf("Config loaded — SUPABASE=%t, CLOUDFLARE=%t, AI=%s, PORT=%s",
		AppConfig.SupabaseURL != "", AppConfig.CloudflareToken != "", redactedAI, AppConfig.Port)

	// If you require some keys to be present for operation, you can enforce here.
	// Example (do not enforce AI key unless required):
	// if AppConfig.SupabaseKey == "" { log.Fatal("SUPABASE_KEY required") }
	// keep behavior permissive by default
}

// Redact returns a redacted version of a string `s`.
// It is used for logging sensitive information, such as API keys.
// For strings longer than 6 characters, it shows the first and last 2 characters.
// For shorter strings, it returns "****".
func Redact(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 6 {
		return "****"
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

// MustGet retrieves the value of an environment variable by its key.
// If the variable is not set or is empty, it logs a fatal error with a
// descriptive message.
//
// Parameters:
//   key: The key of the environment variable.
//   desc: A description of what the variable is used for.
//
// Returns:
//   The value of the environment variable.
func MustGet(key, desc string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required configuration %s (%s) missing", key, desc)
	}
	return v
}
