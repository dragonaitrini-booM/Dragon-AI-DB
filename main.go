/*
Main application entry point with production-grade setup
*/
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"datacentral-tt/aiexecutor"
	"datacentral-tt/persistence"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load configuration from environment
	config := LoadConfig()

	// Initialize Supabase client
	supabaseClient := persistence.NewSupabaseClient(
		config.SupabaseURL,
		config.SupabaseKey,
		logger,
	)

	// Initialize AI executor
	executor := aiexecutor.NewExecutorService(
		config.KimiAPIKey,
		supabaseClient,
		logger,
	)

	// Setup HTTP server with proper handlers
	mux := http.NewServeMux()

	// Main endpoint
	mux.HandleFunc("/api/execute", func(w http.ResponseWriter, r *http.Request) {
		handleExecuteCommand(w, r, executor, logger)
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		handleHealthCheck(w, r, executor, supabaseClient, logger)
	})

	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// pprof endpoint (localhost only)
	go func() {
		// The pprof routes are mounted on the default serve mux.
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			logger.Error("pprof server failed", slog.String("error", err.Error()))
		}
	}()

	// Graceful shutdown
	go func() {
		logger.Info("starting server", slog.String("port", config.Port))
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", slog.String("error", err.Error()))
	}

	logger.Info("server stopped")
}

var (
	sha   = "dev"
	built = "0"
)

// Config holds application configuration
type Config struct {
	KimiAPIKey   string
	SupabaseURL  string
	SupabaseKey  string
	Port         string
	CommitSHA    string
	BuildTime    string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		KimiAPIKey:  mustSecret("KIMI_API_KEY"),   // from K8s secret volume
		SupabaseKey: mustSecret("SUPABASE_KEY"),
		SupabaseURL: mustSecret("SUPABASE_URL"),
		Port:        getEnvOrDefault("PORT", "8080"),
		CommitSHA:   sha,
		BuildTime:   built,
	}
}

func mustSecret(name string) string {
	b, err := os.ReadFile("/run/secrets/" + name)
	if err != nil {
		panic(fmt.Sprintf("secret %s not mounted: %v", name, err))
	}
	return strings.TrimSpace(string(b))
}

func getEnvOrPanic(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic("missing required environment variable: " + key)
	}
	return value
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
