package main

import (
    "bufio"
    "log"
    "os"
    "strings"
)

// Config holds runtime configuration loaded from environment or .env
type Config struct {
    SupabaseURL     string
    SupabaseKey     string
    CloudflareToken string
    AIAPIKey        string
    MasterKey       string
    Port            string
    LogLevel        string
}

// AppConfig is the global configuration instance
var AppConfig Config

// LoadConfig loads environment variables and, if present, a `.env` file in repo root.
// It does not write secrets anywhere; it only validates presence of required values.
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

// Redact returns a version of s with only first and last 2 chars shown for logs
func Redact(s string) string {
    if s == "" {
        return ""
    }
    if len(s) <= 6 {
        return "****"
    }
    return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

// MustGet returns value or logs a fatal error if missing
func MustGet(key, desc string) string {
    v := os.Getenv(key)
    if v == "" {
        log.Fatalf("required configuration %s (%s) missing", key, desc)
    }
    return v
}
