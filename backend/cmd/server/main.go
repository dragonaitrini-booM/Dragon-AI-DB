package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-database-app/internal/ai"
	"ai-database-app/internal/auth"
	"ai-database-app/internal/database"
	"ai-database-app/internal/handlers"
	"ai-database-app/internal/middleware"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

type Config struct {
	Port        string
	DatabaseURL string
	SupabaseURL string
	SupabaseKey string
	HfAPIKey    string
	JWTSecret   string
	RedisURL    string
	Environment string
}

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Load configuration
	config := loadConfig()

	// Initialize database
	db, err := database.NewConnection(config.DatabaseURL)
	if err != nil {
		logger.Error("Failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	// Initialize AI service
	aiService := ai.NewService(config.HfAPIKey, config.RedisURL, logger)

	// Initialize auth service
	authService := auth.NewService(config.JWTSecret, db.DB, logger)

	// Initialize handlers
	h := handlers.New(db.DB, aiService, authService, logger)

	// Setup router
	r := mux.NewRouter()

	// Middleware
	r.Use(middleware.Gzip)
	r.Use(middleware.Logging(logger))
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.RateLimit())

	// Public routes
	r.HandleFunc("/health", h.HealthCheck).Methods("GET")
	r.HandleFunc("/api/auth/login", h.Login).Methods("POST")
	r.HandleFunc("/api/auth/register", h.Register).Methods("POST")

	// Protected routes
	protected := r.PathPrefix("/api").Subrouter()
	protected.Use(middleware.Auth(authService))

	// AI endpoints
	protected.HandleFunc("/execute", h.ExecuteAICommand).Methods("POST")
	protected.HandleFunc("/execute-batch", h.ExecuteBatchCommands).Methods("POST")
	protected.HandleFunc("/query", h.ExecuteQuery).Methods("POST")

	// Data endpoints
	protected.HandleFunc("/databases", h.ListDatabases).Methods("GET")
	protected.HandleFunc("/tables/{database}", h.ListTables).Methods("GET")
	protected.HandleFunc("/schema/{database}/{table}", h.GetTableSchema).Methods("GET")

	// Export endpoints
	protected.HandleFunc("/export/pdf", h.ExportPDF).Methods("POST")
	protected.HandleFunc("/export/excel", h.ExportExcel).Methods("POST")
	protected.HandleFunc("/export/csv", h.ExportCSV).Methods("POST")

	// Dashboard endpoints
	protected.HandleFunc("/dashboards", h.ListDashboards).Methods("GET")
	protected.HandleFunc("/dashboards", h.CreateDashboard).Methods("POST")
	protected.HandleFunc("/dashboards/{id}", h.GetDashboard).Methods("GET")
	protected.HandleFunc("/dashboards/{id}", h.UpdateDashboard).Methods("PUT")
	protected.HandleFunc("/dashboards/{id}", h.DeleteDashboard).Methods("DELETE")

	// History endpoints
	protected.HandleFunc("/history", h.GetHistory).Methods("GET")
	protected.HandleFunc("/history", h.SaveHistory).Methods("POST")

	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	handler := c.Handler(r)

	// Start server
	srv := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("Starting server", slog.String("port", config.Port))
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("Server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", slog.String("error", err.Error()))
	}

	logger.Info("Server stopped")
}

func loadConfig() *Config {
	return &Config{
		Port:        getEnvOrDefault("PORT", "8080"),
		DatabaseURL: getEnvOrPanic("DATABASE_URL"),
		SupabaseURL: getEnvOrPanic("SUPABASE_URL"),
		SupabaseKey: getEnvOrPanic("SUPABASE_ANON_KEY"),
		HfAPIKey:    getEnvOrPanic("HF_API_KEY"),
		JWTSecret:   getEnvOrPanic("JWT_SECRET"),
		RedisURL:    getEnvOrDefault("REDIS_URL", "redis://localhost:6379"),
		Environment: getEnvOrDefault("ENVIRONMENT", "development"),
	}
}

func getEnvOrPanic(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic("Missing required environment variable: " + key)
	}
	return value
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
