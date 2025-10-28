/*
Package persistence handles dual-path data storage with Supabase.
Production-grade error handling and connection pooling.
*/
package persistence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// PathType defines storage destination
type PathType string

const (
	PathPrivate PathType = "private"
	PathPublic  PathType = "public"
)

// SupabaseClient handles database operations
type SupabaseClient struct {
	URL       string
	APIKey    string
	client    *http.Client
	logger    *slog.Logger
}

// NewSupabaseClient creates a production-ready client
func NewSupabaseClient(url, apiKey string, logger *slog.Logger) *SupabaseClient {
	// Production HTTP client with proper timeouts
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &SupabaseClient{
		URL:    url,
		APIKey: apiKey,
		client: client,
		logger: logger,
	}
}

// ContentRecord represents stored content
type ContentRecord struct {
	ID        string    `json:"id,omitempty"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// PersistContent saves content to appropriate path with proper error handling
func (sc *SupabaseClient) PersistContent(ctx context.Context, content, userID string, pathType PathType) error {
	// Input validation
	if content == "" {
		return fmt.Errorf("content cannot be empty")
	}
	if userID == "" {
		return fmt.Errorf("userID cannot be empty")
	}

	tableName := sc.getTableName(userID, pathType)

	record := ContentRecord{
		UserID:    userID,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}

	// Structured logging for observability
	sc.logger.InfoContext(ctx, "persisting content",
		slog.String("table", tableName),
		slog.String("user_id", userID),
		slog.String("path_type", string(pathType)),
		slog.Int("content_length", len(content)),
	)

	if err := sc.insertRecord(ctx, tableName, record); err != nil {
		sc.logger.ErrorContext(ctx, "failed to persist content",
			slog.String("error", err.Error()),
			slog.String("table", tableName),
		)
		return fmt.Errorf("persistence failed: %w", err)
	}

	return nil
}

// insertRecord performs the actual database insert with retry logic
func (sc *SupabaseClient) insertRecord(ctx context.Context, tableName string, record ContentRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	url := fmt.Sprintf("%s/rest/v1/%s", sc.URL, tableName)

	// Create request with context for timeout control
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sc.APIKey)
	req.Header.Set("apikey", sc.APIKey)
	req.Header.Set("Prefer", "return=minimal")

	// Execute with retry logic
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := sc.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed (attempt %d): %w", attempt+1, err)
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil // Success
		}

		lastErr = fmt.Errorf("HTTP %d: request failed (attempt %d)", resp.StatusCode, attempt+1)

		// Don't retry client errors (4xx)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			break
		}

		time.Sleep(time.Duration(attempt+1) * time.Second)
	}

	return lastErr
}

// getTableName returns appropriate table based on path type
func (sc *SupabaseClient) getTableName(userID string, pathType PathType) string {
	switch pathType {
	case PathPrivate:
		return fmt.Sprintf("users_%s_pages", userID)
	case PathPublic:
		return "public_data_pages"
	default:
		return "public_data_pages"
	}
}

// Health check for monitoring
func (sc *SupabaseClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/rest/v1/", sc.URL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+sc.APIKey)
	req.Header.Set("apikey", sc.APIKey)

	resp, err := sc.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("health check failed: HTTP %d", resp.StatusCode)
	}

	return nil
}
