package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"datacentral-tt/aiexecutor"
	"datacentral-tt/persistence"
)

// handleExecuteCommand processes AI command requests
func handleExecuteCommand(
	w http.ResponseWriter,
	r *http.Request,
	executor *aiexecutor.ExecutorService,
	logger *slog.Logger,
) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set request timeout
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Parse request
	var req aiexecutor.ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.WarnContext(ctx, "invalid request body", slog.String("error", err.Error()))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Execute command
	resp, err := executor.ExecuteAICommand(ctx, req)
	if err != nil {
		logger.ErrorContext(ctx, "command execution failed", slog.String("error", err.Error()))
		http.Error(w, "Execution failed", http.StatusInternalServerError)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.ErrorContext(ctx, "failed to encode response", slog.String("error", err.Error()))
	}
}

// handleHealthCheck provides service health status
func handleHealthCheck(
	w http.ResponseWriter,
	r *http.Request,
	executor *aiexecutor.ExecutorService,
	supabase *persistence.SupabaseClient,
	logger *slog.Logger,
) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	health := map[string]string{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Check AI service
	if err := executor.HealthCheck(ctx); err != nil {
		health["ai_service"] = "unhealthy: " + err.Error()
		health["status"] = "degraded"
	} else {
		health["ai_service"] = "healthy"
	}

	// Check Supabase
	if err := supabase.HealthCheck(ctx); err != nil {
		health["database"] = "unhealthy: " + err.Error()
		health["status"] = "degraded"
	} else {
		health["database"] = "healthy"
	}

	statusCode := http.StatusOK
	if health["status"] != "ok" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(health)
}
